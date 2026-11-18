// Package heif is a native module that reads HEIC and HEIF (and AVIF)
// pictures with macOS's own decoder, ImageIO, which Go has none of. Its
// interface is implemented in Swift (see AGENTS.md); elsewhere NewDecoder
// returns errors.ErrUnsupported.
package heif

// Decoder decodes pictures of the HEIF family.
//
//mygo:native
type Decoder interface {
	// ToPNG decodes the picture at path, turned upright as its orientation
	// says, and writes it as a PNG at out.
	ToPNG(path string, out string) error
}
