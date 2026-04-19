package gd

import (
	"github.com/KarpelesLab/gogd"
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpctx"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

// > class GdImage
var GdImage = &phpobj.ZClass{
	Name: "GdImage",
}

func getImg(ctx phpv.Context, z *phpv.ZVal) (*gogd.Image, error) {
	if z == nil || z.GetType() != phpv.ZtObject {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Argument must be of type GdImage")
	}
	obj := z.Value().(*phpobj.ZObject)
	opaque := obj.GetOpaque(GdImage)
	if opaque == nil {
		return nil, phpobj.ThrowError(ctx, phpobj.TypeError, "Argument must be of type GdImage")
	}
	return opaque.(*gogd.Image), nil
}

func wrapImg(ctx phpv.Context, img *gogd.Image) (*phpv.ZVal, error) {
	obj, err := phpobj.NewZObjectOpaque(ctx, GdImage, img)
	if err != nil {
		return nil, err
	}
	return obj.ZVal(), nil
}

func init() {
	phpctx.RegisterExt(&phpctx.Ext{
		Name:    "gd",
		Version: core.VERSION,
		Classes: []*phpobj.ZClass{GdImage},
		Functions: map[string]*phpctx.ExtFunction{
			// create.go
			"imagecreate":          {Func: fncImageCreate, Args: []*phpctx.ExtFunctionArg{{ArgName: "width"}, {ArgName: "height"}}},
			"imagecreatetruecolor": {Func: fncImageCreateTrueColor, Args: []*phpctx.ExtFunctionArg{{ArgName: "width"}, {ArgName: "height"}}},
			"imagedestroy":         {Func: fncImageDestroy, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}}},
			"imagesx":             {Func: fncImageSX, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}}},
			"imagesy":             {Func: fncImageSY, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}}},
			"imageistruecolor":    {Func: fncImageIsTrueColor, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}}},
			// color.go
			"imagecolorallocate":      {Func: fncImageColorAllocate, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "red"}, {ArgName: "green"}, {ArgName: "blue"}}},
			"imagecolorallocatealpha": {Func: fncImageColorAllocateAlpha, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "red"}, {ArgName: "green"}, {ArgName: "blue"}, {ArgName: "alpha"}}},
			"imagecolortransparent":   {Func: fncImageColorTransparent, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "color", Optional: true}}},
			"imagecolorsforindex":     {Func: fncImageColorsForIndex, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "color"}}},
			"imagecolorstotal":        {Func: fncImageColorsTotal, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}}},
			"imagecolordeallocate":    {Func: fncImageColorDeallocate, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "color"}}},
			"imagecolorexact":         {Func: fncImageColorExact, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "red"}, {ArgName: "green"}, {ArgName: "blue"}}},
			"imagecolorclosest":       {Func: fncImageColorClosest, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "red"}, {ArgName: "green"}, {ArgName: "blue"}}},
			"imagecolorresolve":       {Func: fncImageColorResolve, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "red"}, {ArgName: "green"}, {ArgName: "blue"}}},
			// draw.go
			"imagesetpixel":         {Func: fncImageSetPixel, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "x"}, {ArgName: "y"}, {ArgName: "color"}}},
			"imageline":            {Func: fncImageLine, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "x1"}, {ArgName: "y1"}, {ArgName: "x2"}, {ArgName: "y2"}, {ArgName: "color"}}},
			"imagerectangle":       {Func: fncImageRectangle, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "x1"}, {ArgName: "y1"}, {ArgName: "x2"}, {ArgName: "y2"}, {ArgName: "color"}}},
			"imagefilledrectangle": {Func: fncImageFilledRectangle, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "x1"}, {ArgName: "y1"}, {ArgName: "x2"}, {ArgName: "y2"}, {ArgName: "color"}}},
			"imageellipse":         {Func: fncImageEllipse, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "cx"}, {ArgName: "cy"}, {ArgName: "width"}, {ArgName: "height"}, {ArgName: "color"}}},
			"imagefilledellipse":   {Func: fncImageFilledEllipse, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "cx"}, {ArgName: "cy"}, {ArgName: "width"}, {ArgName: "height"}, {ArgName: "color"}}},
			"imagepolygon":         {Func: fncImagePolygon, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "points"}, {ArgName: "color"}}},
			"imagefilledpolygon":   {Func: fncImageFilledPolygon, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "points"}, {ArgName: "color"}}},
			"imagefill":            {Func: fncImageFill, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "x"}, {ArgName: "y"}, {ArgName: "color"}}},
			"imagesetthickness":    {Func: fncImageSetThickness, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "thickness"}}},
			"imagearc":             {Func: fncImageArc, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "cx"}, {ArgName: "cy"}, {ArgName: "width"}, {ArgName: "height"}, {ArgName: "start"}, {ArgName: "end"}, {ArgName: "color"}}},
			"imagefilledarc":       {Func: fncImageFilledArc, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "cx"}, {ArgName: "cy"}, {ArgName: "width"}, {ArgName: "height"}, {ArgName: "start"}, {ArgName: "end"}, {ArgName: "color"}, {ArgName: "style"}}},
			"imagealphablending":   {Func: fncImageAlphaBlending, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "enable"}}},
			"imagesavealpha":       {Func: fncImageSaveAlpha, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "enable"}}},
			"imagesetclip":         {Func: fncImageSetClip, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "x1"}, {ArgName: "y1"}, {ArgName: "x2"}, {ArgName: "y2"}}},
			"imagegetclip":         {Func: fncImageGetClip, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}}},
			"imagecolorat":         {Func: fncImageColorAt, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "x"}, {ArgName: "y"}}},
			// text.go
			"imagestring":   {Func: fncImageString, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "font"}, {ArgName: "x"}, {ArgName: "y"}, {ArgName: "string"}, {ArgName: "color"}}},
			"imagestringup": {Func: fncImageStringUp, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "font"}, {ArgName: "x"}, {ArgName: "y"}, {ArgName: "string"}, {ArgName: "color"}}},
			"imagettftext":  {Func: fncImageTTFText, Args: []*phpctx.ExtFunctionArg{{ArgName: "size"}, {ArgName: "angle"}, {ArgName: "x"}, {ArgName: "y"}, {ArgName: "color"}, {ArgName: "font_filename"}, {ArgName: "text"}}},
			"imagettfbbox":  {Func: fncImageTTFBBox, Args: []*phpctx.ExtFunctionArg{{ArgName: "size"}, {ArgName: "angle"}, {ArgName: "font_filename"}, {ArgName: "text"}}},
			"imagefontwidth":  {Func: fncImageFontWidth, Args: []*phpctx.ExtFunctionArg{{ArgName: "font"}}},
			"imagefontheight": {Func: fncImageFontHeight, Args: []*phpctx.ExtFunctionArg{{ArgName: "font"}}},
			// io.go
			"imagecreatefromstring": {Func: fncImageCreateFromString, Args: []*phpctx.ExtFunctionArg{{ArgName: "data"}}},
			"imagecreatefrompng":    {Func: fncImageCreateFromPNG, Args: []*phpctx.ExtFunctionArg{{ArgName: "filename"}}},
			"imagecreatefromjpeg":   {Func: fncImageCreateFromJPEG, Args: []*phpctx.ExtFunctionArg{{ArgName: "filename"}}},
			"imagecreatefromgif":    {Func: fncImageCreateFromGIF, Args: []*phpctx.ExtFunctionArg{{ArgName: "filename"}}},
			"imagecreatefrombmp":    {Func: fncImageCreateFromBMP, Args: []*phpctx.ExtFunctionArg{{ArgName: "filename"}}},
			"imagepng":  {Func: fncImagePNG, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "file", Optional: true}, {ArgName: "quality", Optional: true}, {ArgName: "filters", Optional: true}}},
			"imagejpeg": {Func: fncImageJPEG, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "file", Optional: true}, {ArgName: "quality", Optional: true}}},
			"imagegif":  {Func: fncImageGIF, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "file", Optional: true}}},
			"imagebmp":  {Func: fncImageBMP, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "file", Optional: true}, {ArgName: "compressed", Optional: true}}},
			// transform.go
			"imagecopy":           {Func: fncImageCopy, Args: []*phpctx.ExtFunctionArg{{ArgName: "dst_image"}, {ArgName: "src_image"}, {ArgName: "dst_x"}, {ArgName: "dst_y"}, {ArgName: "src_x"}, {ArgName: "src_y"}, {ArgName: "src_width"}, {ArgName: "src_height"}}},
			"imagecopyresized":    {Func: fncImageCopyResized, Args: []*phpctx.ExtFunctionArg{{ArgName: "dst_image"}, {ArgName: "src_image"}, {ArgName: "dst_x"}, {ArgName: "dst_y"}, {ArgName: "src_x"}, {ArgName: "src_y"}, {ArgName: "dst_width"}, {ArgName: "dst_height"}, {ArgName: "src_width"}, {ArgName: "src_height"}}},
			"imagecopyresampled":  {Func: fncImageCopyResampled, Args: []*phpctx.ExtFunctionArg{{ArgName: "dst_image"}, {ArgName: "src_image"}, {ArgName: "dst_x"}, {ArgName: "dst_y"}, {ArgName: "src_x"}, {ArgName: "src_y"}, {ArgName: "dst_width"}, {ArgName: "dst_height"}, {ArgName: "src_width"}, {ArgName: "src_height"}}},
			"imagecopymerge":      {Func: fncImageCopyMerge, Args: []*phpctx.ExtFunctionArg{{ArgName: "dst_image"}, {ArgName: "src_image"}, {ArgName: "dst_x"}, {ArgName: "dst_y"}, {ArgName: "src_x"}, {ArgName: "src_y"}, {ArgName: "src_width"}, {ArgName: "src_height"}, {ArgName: "pct"}}},
			"imagerotate":         {Func: fncImageRotate, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "angle"}, {ArgName: "background_color"}}},
			"imagecrop":           {Func: fncImageCrop, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "rectangle"}}},
			"imagescale":          {Func: fncImageScale, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "width"}, {ArgName: "height", Optional: true}, {ArgName: "mode", Optional: true}}},
			"imageflip":           {Func: fncImageFlip, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "mode"}}},
			// info.go
			"gd_info":         {Func: fncGDInfo},
			"imagetypes":      {Func: fncImageTypes},
			"imageresolution": {Func: fncImageResolution, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "resolution_x", Optional: true}, {ArgName: "resolution_y", Optional: true}}},
			// filter.go
			"imagefilter":     {Func: fncImageFilter, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "filter"}, {ArgName: "args", Optional: true}}},
			"imageconvolution": {Func: fncImageConvolution, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "matrix"}, {ArgName: "divisor"}, {ArgName: "offset"}}},
			"imagegammacorrect": {Func: fncImageGammaCorrect, Args: []*phpctx.ExtFunctionArg{{ArgName: "image"}, {ArgName: "input_gamma"}, {ArgName: "output_gamma"}}},
		},
		Constants: map[phpv.ZString]phpv.Val{
			// Image format bitmask constants (IMG_*)
			"IMG_GIF":  phpv.ZInt(gogd.ImgGIF),
			"IMG_JPG":  phpv.ZInt(gogd.ImgJPEG),
			"IMG_JPEG": phpv.ZInt(gogd.ImgJPEG),
			"IMG_PNG":  phpv.ZInt(gogd.ImgPNG),
			"IMG_WBMP": phpv.ZInt(gogd.ImgWBMP),
			"IMG_XPM":  phpv.ZInt(gogd.ImgXPM),
			"IMG_WEBP": phpv.ZInt(gogd.ImgWEBP),
			"IMG_BMP":  phpv.ZInt(gogd.ImgBMP),
			"IMG_TGA":  phpv.ZInt(gogd.ImgTGA),
			"IMG_AVIF": phpv.ZInt(gogd.ImgAVIF),

			// IMAGETYPE_* constants
			"IMAGETYPE_GIF":  phpv.ZInt(gogd.ImageTypeGIF),
			"IMAGETYPE_JPEG": phpv.ZInt(gogd.ImageTypeJPEG),
			"IMAGETYPE_PNG":  phpv.ZInt(gogd.ImageTypePNG),
			"IMAGETYPE_BMP":  phpv.ZInt(gogd.ImageTypeBMP),
			"IMAGETYPE_WBMP": phpv.ZInt(gogd.ImageTypeWBMP),
			"IMAGETYPE_XBM":  phpv.ZInt(gogd.ImageTypeXBM),
			"IMAGETYPE_WEBP": phpv.ZInt(gogd.ImageTypeWEBP),
			"IMAGETYPE_AVIF": phpv.ZInt(gogd.ImageTypeAVIF),

			// Color constants
			"IMG_COLOR_STYLED":        phpv.ZInt(gogd.ColorStyled),
			"IMG_COLOR_BRUSHED":       phpv.ZInt(gogd.ColorBrushed),
			"IMG_COLOR_STYLEDBRUSHED": phpv.ZInt(gogd.ColorStyledBrushed),
			"IMG_COLOR_TILED":         phpv.ZInt(gogd.ColorTiled),
			"IMG_COLOR_TRANSPARENT":   phpv.ZInt(gogd.ColorTransparent),

			// Arc style constants
			"IMG_ARC_ROUNDED": phpv.ZInt(0),
			"IMG_ARC_PIE":     phpv.ZInt(gogd.ImgArcPie),
			"IMG_ARC_CHORD":   phpv.ZInt(gogd.ImgArcChord),
			"IMG_ARC_NOFILL":  phpv.ZInt(gogd.ImgArcNoFill),
			"IMG_ARC_EDGED":   phpv.ZInt(gogd.ImgArcEdged),

			// Flip constants
			"IMG_FLIP_HORIZONTAL": phpv.ZInt(gogd.ImgFlipHorizontal),
			"IMG_FLIP_VERTICAL":   phpv.ZInt(gogd.ImgFlipVertical),
			"IMG_FLIP_BOTH":       phpv.ZInt(gogd.ImgFlipHorizontal | gogd.ImgFlipVertical),

			// Filter constants
			"IMG_FILTER_NEGATE":         phpv.ZInt(gogd.FilterNegate),
			"IMG_FILTER_GRAYSCALE":      phpv.ZInt(gogd.FilterGrayscale),
			"IMG_FILTER_BRIGHTNESS":     phpv.ZInt(gogd.FilterBrightness),
			"IMG_FILTER_CONTRAST":       phpv.ZInt(gogd.FilterContrast),
			"IMG_FILTER_COLORIZE":       phpv.ZInt(gogd.FilterColorize),
			"IMG_FILTER_EDGEDETECT":     phpv.ZInt(gogd.FilterEdgeDetect),
			"IMG_FILTER_EMBOSS":         phpv.ZInt(gogd.FilterEmboss),
			"IMG_FILTER_GAUSSIAN_BLUR":  phpv.ZInt(gogd.FilterGaussianBlur),
			"IMG_FILTER_SELECTIVE_BLUR": phpv.ZInt(gogd.FilterSelectiveBlur),
			"IMG_FILTER_MEAN_REMOVAL":   phpv.ZInt(gogd.FilterMeanRemoval),
			"IMG_FILTER_SMOOTH":         phpv.ZInt(gogd.FilterSmooth),
			"IMG_FILTER_PIXELATE":       phpv.ZInt(gogd.FilterPixelate),
			"IMG_FILTER_SCATTER":        phpv.ZInt(gogd.FilterScatter),

			// GD version
			"GD_VERSION":         phpv.ZString("2.3.3"),
			"GD_MAJOR_VERSION":   phpv.ZInt(2),
			"GD_MINOR_VERSION":   phpv.ZInt(3),
			"GD_RELEASE_VERSION": phpv.ZInt(3),
			"GD_EXTRA_VERSION":   phpv.ZString(""),
		},
	})
}
