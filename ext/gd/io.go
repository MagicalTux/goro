package gd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/KarpelesLab/gogd"
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpobj"
	"github.com/KarpelesLab/goro/core/phpv"
)

func fncImageCreateFromString(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var data phpv.ZString
	_, err := core.Expand(ctx, args, &data)
	if err != nil {
		return nil, err
	}
	img, err := gogd.ImageCreateFromString([]byte(data))
	if err != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return wrapImg(ctx, img)
}

func fncImageCreateFromPNG(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return imageCreateFromFile(ctx, args, gogd.ImageCreateFromPNG, "imagecreatefrompng")
}

func fncImageCreateFromJPEG(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return imageCreateFromFile(ctx, args, gogd.ImageCreateFromJPEG, "imagecreatefromjpeg")
}

func fncImageCreateFromGIF(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return imageCreateFromFile(ctx, args, gogd.ImageCreateFromGIF, "imagecreatefromgif")
}

func fncImageCreateFromBMP(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return imageCreateFromFile(ctx, args, gogd.ImageCreateFromBMP, "imagecreatefrombmp")
}

func imageCreateFromFile(ctx phpv.Context, args []*phpv.ZVal, decoder func(io.Reader) (*gogd.Image, error), funcName string) (*phpv.ZVal, error) {
	var filename phpv.ZString
	_, err := core.Expand(ctx, args, &filename)
	if err != nil {
		return nil, err
	}
	fname := string(filename)
	if !filepath.IsAbs(fname) {
		fname = filepath.Join(string(ctx.Global().Getwd()), fname)
	}
	f, err := os.Open(fname)
	if err != nil {
		ctx.Warn("%s(%s): Failed to open stream: %s", funcName, filename, err.Error())
		return phpv.ZFalse.ZVal(), nil
	}
	defer f.Close()
	img, err := decoder(f)
	if err != nil {
		ctx.Warn("%s(): %s is not a valid image file", funcName, filename)
		return phpv.ZFalse.ZVal(), nil
	}
	return wrapImg(ctx, img)
}

// imagepng($image, $file=null, $quality=-1, $filters=-1)
func fncImagePNG(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	_, err := core.Expand(ctx, args, &img)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return outputImage(ctx, args, func(buf *bytes.Buffer) error {
		return gogd.ImagePNG(gdImg, buf)
	})
}

// imagejpeg($image, $file=null, $quality=-1)
func fncImageJPEG(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var quality *phpv.ZInt
	_, err := core.Expand(ctx, args, &img, nil, &quality)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	q := -1
	if quality != nil {
		q = int(*quality)
	}
	return outputImage(ctx, args, func(buf *bytes.Buffer) error {
		return gogd.ImageJPEG(gdImg, buf, q)
	})
}

// imagegif($image, $file=null)
func fncImageGIF(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	_, err := core.Expand(ctx, args, &img)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return outputImage(ctx, args, func(buf *bytes.Buffer) error {
		return gogd.ImageGIF(gdImg, buf)
	})
}

// imagebmp($image, $file=null, $compressed=true)
func fncImageBMP(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	_, err := core.Expand(ctx, args, &img)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return outputImage(ctx, args, func(buf *bytes.Buffer) error {
		return gogd.ImageBMP(gdImg, buf)
	})
}

// outputImage handles the optional file parameter for image output functions.
// If args[1] is provided and non-null, write to that file. Otherwise write to output.
func outputImage(ctx phpv.Context, args []*phpv.ZVal, encode func(*bytes.Buffer) error) (*phpv.ZVal, error) {
	var buf bytes.Buffer
	if err := encode(&buf); err != nil {
		return nil, phpobj.ThrowError(ctx, phpobj.Error, err.Error())
	}

	// Check for optional filename parameter (args[1])
	if len(args) > 1 && args[1] != nil && !args[1].IsNull() {
		fname := string(args[1].AsString(ctx))
		if !filepath.IsAbs(fname) {
			fname = filepath.Join(string(ctx.Global().Getwd()), fname)
		}
		if strings.HasSuffix(fname, string(os.PathSeparator)) {
			return phpv.ZFalse.ZVal(), nil
		}
		if err := os.WriteFile(fname, buf.Bytes(), 0644); err != nil {
			ctx.Warn("Failed to write image: %s", err.Error())
			return phpv.ZFalse.ZVal(), nil
		}
		return phpv.ZTrue.ZVal(), nil
	}

	// Write to output buffer
	ctx.Write(buf.Bytes())
	return phpv.ZTrue.ZVal(), nil
}
