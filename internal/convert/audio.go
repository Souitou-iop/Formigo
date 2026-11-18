package convert

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/egoist/mygo/audio"
	_ "github.com/egoist/mygo/audio/flac"
	_ "github.com/egoist/mygo/audio/mp3"
	_ "github.com/egoist/mygo/audio/vorbis"
	"github.com/mewkiz/flac"
	"github.com/mewkiz/flac/frame"
	"github.com/mewkiz/flac/meta"
)

// AudioInfo returns how long a sound lasts and its channels and rate.
func AudioInfo(path string) (d time.Duration, channels, rate int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, 0, err
	}
	defer f.Close()
	dec, _, err := audio.NewDecoder(f)
	if err != nil {
		return 0, 0, 0, err
	}
	if n := dec.Len(); n > 0 {
		d = time.Duration(float64(n) / float64(dec.SampleRate()) * float64(time.Second))
	}
	return d, dec.Channels(), dec.SampleRate(), nil
}

// sampleSink takes interleaved integer samples, a block at a time.
type sampleSink interface {
	write(samples []int32, channels int) error
	close() error
}

func convertAudio(ctx context.Context, src string, w io.Writer, to string, o Options, p Progress) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	dec, _, err := audio.NewDecoder(f)
	if err != nil {
		if errors.Is(err, audio.ErrFormat) {
			return ErrUnsupported
		}
		return fmt.Errorf("无法读取音频：%w", err)
	}
	inCh := dec.Channels()
	if inCh < 1 {
		return errors.New("音频没有声道")
	}
	outCh := min(inCh, 2)
	if o.Mono {
		outCh = 1
	}
	bits := 16
	if o.BitDepth == 24 {
		bits = 24
	}
	rate := dec.SampleRate()
	var sink sampleSink
	switch to {
	case "wav":
		ws, ok := w.(io.WriteSeeker)
		if !ok {
			return errors.New("WAV 需要可定位的输出")
		}
		sink, err = newWAV(ws, rate, outCh, bits)
	case "flac":
		// The encoder seeks back to write the stream's length and checksum,
		// and closes what it was given, which is the caller's to close.
		if ws, ok := w.(io.WriteSeeker); ok {
			w = writeSeeker{ws}
		}
		sink, err = newFLAC(w, rate, outCh, bits, dec.Len())
	default:
		return ErrUnsupported
	}
	if err != nil {
		return err
	}
	const block = 4096
	buf := make([]float32, block*inCh)
	out := make([]int32, block*outCh)
	scale := float64(int64(1)<<(bits-1) - 1)
	total := dec.Len()
	var done int64
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, rerr := dec.Read(buf)
		frames := n / inCh
		for i := 0; i < frames; i++ {
			in := buf[i*inCh : i*inCh+inCh]
			for c := 0; c < outCh; c++ {
				var v float64
				switch {
				case outCh == 1:
					for _, s := range in {
						v += float64(s)
					}
					v /= float64(len(in))
				default:
					v = float64(in[min(c, inCh-1)])
				}
				out[i*outCh+c] = int32(math.Round(max(-1, min(1, v)) * scale))
			}
		}
		if frames > 0 {
			if err := sink.write(out[:frames*outCh], outCh); err != nil {
				return err
			}
			done += int64(frames)
			if total > 0 {
				p(0.05 + 0.9*float64(done)/float64(total))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fmt.Errorf("解码失败：%w", rerr)
		}
		if n == 0 {
			break
		}
	}
	return sink.close()
}

// wavSink writes PCM WAV, patching its sizes once the length is known.
type wavSink struct {
	w        io.WriteSeeker
	bw       *bufio.Writer
	bytesPer int
	n        int64
	tmp      []byte
}

func newWAV(w io.WriteSeeker, rate, channels, bits int) (*wavSink, error) {
	s := &wavSink{w: w, bw: bufio.NewWriterSize(w, 1<<16), bytesPer: bits / 8}
	hdr := make([]byte, 44)
	copy(hdr, "RIFF")
	copy(hdr[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(hdr[16:], 16)
	binary.LittleEndian.PutUint16(hdr[20:], 1)
	binary.LittleEndian.PutUint16(hdr[22:], uint16(channels))
	binary.LittleEndian.PutUint32(hdr[24:], uint32(rate))
	binary.LittleEndian.PutUint32(hdr[28:], uint32(rate*channels*s.bytesPer))
	binary.LittleEndian.PutUint16(hdr[32:], uint16(channels*s.bytesPer))
	binary.LittleEndian.PutUint16(hdr[34:], uint16(bits))
	copy(hdr[36:], "data")
	_, err := s.bw.Write(hdr)
	return s, err
}

func (s *wavSink) write(samples []int32, _ int) error {
	need := len(samples) * s.bytesPer
	if cap(s.tmp) < need {
		s.tmp = make([]byte, need)
	}
	b := s.tmp[:need]
	for i, v := range samples {
		o := i * s.bytesPer
		b[o] = byte(v)
		b[o+1] = byte(v >> 8)
		if s.bytesPer == 3 {
			b[o+2] = byte(v >> 16)
		}
	}
	s.n += int64(need)
	_, err := s.bw.Write(b)
	return err
}

func (s *wavSink) close() error {
	if s.n > math.MaxUint32-36 {
		return errors.New("音频太长，超过 WAV 的 4 GB 上限")
	}
	if err := s.bw.Flush(); err != nil {
		return err
	}
	var b [4]byte
	for _, patch := range []struct {
		at int64
		v  uint32
	}{{4, uint32(36 + s.n)}, {40, uint32(s.n)}} {
		binary.LittleEndian.PutUint32(b[:], patch.v)
		if _, err := s.w.Seek(patch.at, io.SeekStart); err != nil {
			return err
		}
		if _, err := s.w.Write(b[:]); err != nil {
			return err
		}
	}
	_, err := s.w.Seek(0, io.SeekEnd)
	return err
}

// writeSeeker hides a file's Close from the FLAC encoder.
type writeSeeker struct{ io.WriteSeeker }

// flacSink encodes FLAC frames of a fixed block size, letting the encoder
// choose each subframe's prediction.
type flacSink struct {
	enc     *flac.Encoder
	rate    int
	bits    int
	ch      int
	pending [][]int32
}

const flacBlock = 4096

func newFLAC(w io.Writer, rate, channels, bits int, frames int64) (*flacSink, error) {
	info := &meta.StreamInfo{
		BlockSizeMin:  16,
		BlockSizeMax:  flacBlock,
		SampleRate:    uint32(rate),
		NChannels:     uint8(channels),
		BitsPerSample: uint8(bits),
	}
	if frames > 0 {
		info.NSamples = uint64(frames)
	}
	enc, err := flac.NewEncoder(w, info)
	if err != nil {
		return nil, err
	}
	return &flacSink{enc: enc, rate: rate, bits: bits, ch: channels, pending: make([][]int32, channels)}, nil
}

func (s *flacSink) write(samples []int32, channels int) error {
	for i, v := range samples {
		c := i % channels
		s.pending[c] = append(s.pending[c], v)
	}
	for len(s.pending[0]) >= flacBlock {
		if err := s.flush(flacBlock); err != nil {
			return err
		}
	}
	return nil
}

func (s *flacSink) flush(n int) error {
	chans := frame.ChannelsMono
	if s.ch == 2 {
		chans = frame.ChannelsLR
	}
	fr := &frame.Frame{Header: frame.Header{
		HasFixedBlockSize: true,
		BlockSize:         uint16(n),
		SampleRate:        uint32(s.rate),
		Channels:          chans,
		BitsPerSample:     uint8(s.bits),
	}}
	for c := 0; c < s.ch; c++ {
		smp := append([]int32(nil), s.pending[c][:n]...)
		fr.Subframes = append(fr.Subframes, &frame.Subframe{
			SubHeader: frame.SubHeader{Pred: frame.PredVerbatim},
			Samples:   smp,
			NSamples:  n,
		})
		s.pending[c] = s.pending[c][n:]
	}
	return s.enc.WriteFrame(fr)
}

func (s *flacSink) close() error {
	if n := len(s.pending[0]); n > 0 {
		if err := s.flush(n); err != nil {
			return err
		}
	}
	return s.enc.Close()
}

func init() {
	// Go's own decoders and encoders: what FFmpeg does better, when it is
	// there, at a lower cost.
	register(&edge{engine: "audio", from: []string{"mp3", "ogg", "flac", "wav"}, to: []string{"wav", "flac"}, cost: 3,
		run: func(j *job) error {
			return writeVia(j, func(f *os.File) error { return convertAudio(j.ctx, j.src, f, j.to.ID, j.o, j.p) })
		}})
}
