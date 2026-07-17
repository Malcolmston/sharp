// Library content for the sharp documentation site. Mirrors the shape used by
// the malcolmston/go landing site's data.ts so the sibling sites stay in sync.
export interface Lib {
  id: string; name: string; icon: string; accent: string; pkg: string; node: string;
  repo: string; docs: string; tagline: string; blurb: string; tags: string[];
  features: string[]; node_code: string; go_code: string; integrate: string;
}

export const NODE_ACCENT = '#8cc84b';

export const SHARP: Lib = {
  id:"sharp", name:"sharp", icon:'<i class="fa-solid fa-wand-magic-sparkles"></i>', accent:"#14b8a6",
  pkg:"github.com/malcolmston/sharp", node:"lovell/sharp",
  repo:"https://github.com/malcolmston/sharp", docs:"https://malcolmston.github.io/sharp/",
  tagline:"Fluent, chainable image processing for Go.",
  blurb:"A from-scratch, standard-library-only Go take on the ergonomics of Node's sharp: build a "+
    "Pipeline, chain operations, export to a format. Everything runs on the Go standard library "+
    "(image, image/color, image/png, image/jpeg and math) over an in-memory RGBA buffer — no cgo, "+
    "no third-party dependencies. Construct a pipeline from an image, file, byte slice or reader; the "+
    "source is copied on construction and never mutated. Operations apply eagerly and chain by "+
    "returning the same *Pipeline: resize with FitExact/FitContain/FitCover and Nearest/Bilinear "+
    "sampling, crop/extract, extend, rotate and flip, a full colour suite (grayscale, negate, tint, "+
    "brightness, contrast, gamma, saturation, threshold), separable-Gaussian blur, sharpen and generic "+
    "convolution, alpha-blended compositing and flatten, plus PNG/JPEG in and out. A deferred error "+
    "model retains the first failure and turns later steps into no-ops, so it surfaces once from the "+
    "terminal export or from Err.",
  tags:["fluent pipeline","resize","crop/extract","rotate/flip","colour ops","blur/sharpen","convolution","composite","PNG/JPEG I/O","deferred errors","zero deps","deterministic"],
  features:[
    "Fluent <code>Pipeline</code> built with <code>New</code>, <code>FromFile</code>, <code>FromBytes</code> or <code>FromReader</code> — copy-on-construct, so the source is never mutated",
    "Resize via <code>Resize(ResizeOptions{…})</code> and <code>ResizeTo</code> — <code>FitExact</code>/<code>FitContain</code>/<code>FitCover</code> with <code>Nearest</code> or <code>Bilinear</code> sampling",
    "Region &amp; layout — <code>Crop</code>/<code>Extract</code>, <code>Extend</code> (pad with a fill colour), <code>Rotate90</code>/<code>180</code>/<code>270</code>, arbitrary <code>Rotate</code>, <code>FlipVertical</code>/<code>FlipHorizontal</code> (aka <code>Flip</code>/<code>Flop</code>)",
    "Colour suite — <code>Grayscale</code>, <code>Negate</code>/<code>Invert</code>, <code>Tint</code>, <code>Brightness</code>, <code>Contrast</code>, <code>Gamma</code>, <code>Saturation</code>, <code>Threshold</code>",
    "Convolution — separable-Gaussian <code>Blur</code>, <code>Sharpen</code>, and a generic <code>Convolve</code> over <code>NewKernel</code>",
    "Composition — <code>Composite</code> with alpha blending plus <code>Gravity</code>/offset placement, and <code>Flatten</code> onto a solid background",
    "PNG + JPEG I/O — <code>ToPNG</code>, <code>ToJPEG</code>, <code>ToImage</code>, <code>ToFile</code> with <code>PNGOptions</code>/<code>JPEGOptions</code> quality control",
    "Deferred error model — the first failure is retained, later steps become no-ops, and it surfaces from the terminal call or <code>Err</code>",
    "Introspection — <code>Metadata</code> (width/height/format) and <code>Stats</code> (per-channel means), plus <code>Clone</code> to branch a pipeline",
    "Zero dependencies — pure Go standard library, nothing to audit but the toolchain"
  ],
  node_code:
`const sharp = require("sharp");

sharp("input.jpg")
  .resize(800, null, { fit: "inside" })
  .grayscale()
  .sharpen(1)
  .jpeg({ quality: 85 })
  .toFile("output.jpg");`,
  go_code:
`import "github.com/malcolmston/sharp"

buf, _ := sharp.FromFile("input.jpg").
	Resize(sharp.ResizeOptions{Width: 800, Fit: sharp.FitContain}).
	Grayscale().
	Blur(2).
	ToJPEG(85)
_ = sharp.FromFile("input.jpg").ToFile("output.jpg", sharp.FormatJPEG, 85)`,
  integrate:
`<span class="tok-c">// Build a pipeline from a file. The source image is copied on</span>
<span class="tok-c">// construction, so the file on disk is never touched.</span>
p := sharp.FromFile("photo.png").
	Resize(sharp.ResizeOptions{Width: 1200, Height: 630, Fit: sharp.FitCover, Interpolation: sharp.Bilinear}).
	Brightness(1.05).
	Contrast(1.1).
	Sharpen(0.8)

<span class="tok-c">// Branch the partially-built pipeline into an independent copy and</span>
<span class="tok-c">// produce a grayscale thumbnail without disturbing the original.</span>
thumb, _ := p.Clone().ResizeTo(320, 168).Grayscale().ToPNG()

<span class="tok-c">// Drop a watermark into the corner with alpha blending, flatten any</span>
<span class="tok-c">// transparency onto white, then encode to JPEG. The deferred error</span>
<span class="tok-c">// model means a single check at the end covers every step.</span>
logo, _ := sharp.FromFile("logo.png").ToImage()
buf, err := p.
	Composite(logo, sharp.CompositeOptions{UseGravity: true, Gravity: sharp.GravityBottomRight, Opacity: 0.8}).
	Flatten(sharp.White).
	ToJPEG(90)
if err != nil {
	log.Fatal(err)
}
_ = os.WriteFile("out.jpg", buf, 0o644)
_ = thumb`
};
