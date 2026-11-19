// Local preview of the deck: joins the slides, maps uploaded blobs to local files.
import fs from 'node:fs'
const root = new URL('../deck/project/', import.meta.url)
const P = '/Volumes/SanDisk/Projects/Formigo/'
const blobs = {
 '7f3f23d6e4c3c9ed6298bbed698ef8d5':'presentation/mascot/poses/01-wave.png','5bf3e90629089e9d7b1342ff1f0da6c5':'presentation/mascot/poses/02-peek.png',
 'd2574e1f8a651f7d621619a2301e1680':'presentation/mascot/poses/03-surprise.png','6d9a47909777d6057fa58ace38f14fe2':'presentation/mascot/poses/04-juggle.png',
 '81c8d9312d9475ca913635cc1bdfc7a8':'presentation/mascot/poses/05-point.png','a00aeaf463c6d970e62f413b571b0ec4':'presentation/mascot/poses/06-shield.png',
 '4e5842d2caed032c57cfbc601bbda79f':'presentation/mascot/poses/07-think.png','0ad215d44dca85b1fa218981ce651658':'presentation/mascot/poses/08-cheer.png',
 'fd6df7cf1c5110f106f511d3ae2b98ec':'presentation/mascot/poses/09-laptop.png','5e8824093b4b7e234d656f5f8a713f38':'presentation/mascot/poses/10-run.png',
 '034662701bdad6513dad598affd5cee9':'presentation/mascot/poses/11-devices.png','e9d632955b1cd75bb94d259e23e4ac23':'presentation/mascot/poses/12-palette.png',
 '691b2bf167591df6af666fc805288c02':'presentation/mascot/poses/13-magnifier.png','0cae2f8dd7db35856e2ad43f193489b2':'presentation/mascot/poses/14-bow.png',
 'd034c826540d6e8595501e4b40f47c8f':'presentation/mascot/poses/15-sleep.png','c966794ff0aba3abf4e326b93585e38a':'presentation/mascot/poses/16-dive.png',
 'dcdc4fc7c7057247017209f0f109aba9':'presentation/mascot/poses/17-surf.png','874dbccc985f9d59c53b4360a12b2fe3':'presentation/mascot/poses/18-throw.png',
 '878baf952eccf92e9e75c3f7bf1c5907':'assets/mascot.png',
 '3416bc3c21fc6acb584c376962249131':'docs/screenshots/desktop-hero-light.png','205de8316ff4b21a2e358e5238bcde4c':'docs/screenshots/desktop-queue-light.png',
 'd5283ab8095ba28d34a01aa0e599bb3d':'docs/screenshots/desktop-settings-light.png','e9e4b449c86681755b61c4d21de08cde':'docs/screenshots/android-light.png',
 '807e6f7e852b0b63c9cba5266e6436cb':'docs/screenshots/android-home.png','12e743b2f47609ed00ac265a0275f1f0':'docs/screenshots/ios-home.png',
 '74872bfa53bcb3888e00ce9d1b0862cc':'docs/screenshots/ios-orange.png','17d9b1eb1570f4a8bd77b4cf9bf4da2a':'docs/screenshots/ios-settings.png',
 'acfe9da9eebf9015af876a98e8b588af':'docs/screenshots/desktop-hero-dark.png',
 'fcfdc1e5ff1554914a6af25f0523cd3f':'docs/screenshots/desktop-hero-light.png','f544ab0290385acf2cb03e30e0997d01':'docs/screenshots/desktop-queue-light.png',
 '4a1a8a26de9283f2d31121ff4b6fd2d3':'presentation/deck/assets/android-neutral.jpg','06c126f34b8294a9340edef525e5873e':'presentation/deck/assets/ios-neutral.webp',
}
const deck = JSON.parse(fs.readFileSync(new URL('deck.json', root)))
const css = fs.readFileSync(new URL('deck.css', root), 'utf8')
const only = process.argv[2]
let slides = (only ? only.split(',') : deck.order).map(id => fs.readFileSync(new URL(`slides/${id}.html`, root), 'utf8')).join('\n')
slides = slides.replace(/\/_blob\/([0-9a-f]{32})/g, (m, id) => blobs[id] ? 'file://' + P + blobs[id] : m)
const viewer = `* { margin: 0 } body { font: 32px/1.2 sans-serif; background:#888 } body > section { width: 1920px !important; height: 1080px !important; overflow: hidden !important; box-sizing: border-box !important; position: relative; isolation: isolate; margin-bottom:20px }
aside { display: none !important } x-shape, x-icon, x-embed { display: block }`
const fonts = Object.values(deck.faces).map(f => `<link rel="stylesheet" href="${f.href}">`).join('')
fs.writeFileSync(new URL('./index.html', import.meta.url), `<!doctype html><meta charset="utf-8"><title>Deck preview</title>${fonts}<style>${viewer}</style><style>${css}</style><body style="zoom:${process.argv[3]||0.5}">${slides}<script>document.querySelectorAll('.fragment').forEach(e=>e.classList.add('visible'))</script>`)
