package gd

import (
	"github.com/KarpelesLab/gogd"
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

func fncImageString(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var font, x, y phpv.ZInt
	var s phpv.ZString
	var color phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &font, &x, &y, &s, &color)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageString(gdImg, int(font), int(x), int(y), string(s), gogd.Color(color))).ZVal(), nil
}

func fncImageStringUp(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var font, x, y phpv.ZInt
	var s phpv.ZString
	var color phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &font, &x, &y, &s, &color)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageStringUp(gdImg, int(font), int(x), int(y), string(s), gogd.Color(color))).ZVal(), nil
}

func fncImageTTFText(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var size phpv.ZFloat
	var angle phpv.ZFloat
	var x, y, color phpv.ZInt
	var fontFile, text phpv.ZString
	_, err := core.Expand(ctx, args, &img, &size, &angle, &x, &y, &color, &fontFile, &text)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	bbox, ttfErr := gogd.ImageTTFText(gdImg, float64(size), float64(angle), int(x), int(y), gogd.Color(color), string(fontFile), string(text))
	if ttfErr != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	result := phpv.NewZArray()
	for i := 0; i < 8; i++ {
		result.OffsetSet(ctx, nil, phpv.ZInt(bbox[i]).ZVal())
	}
	return result.ZVal(), nil
}

func fncImageTTFBBox(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var size, angle phpv.ZFloat
	var fontFile, text phpv.ZString
	_, err := core.Expand(ctx, args, &size, &angle, &fontFile, &text)
	if err != nil {
		return nil, err
	}
	bbox, ttfErr := gogd.ImageTTFBBox(float64(size), float64(angle), string(fontFile), string(text))
	if ttfErr != nil {
		return phpv.ZFalse.ZVal(), nil
	}
	result := phpv.NewZArray()
	for i := 0; i < 8; i++ {
		result.OffsetSet(ctx, nil, phpv.ZInt(bbox[i]).ZVal())
	}
	return result.ZVal(), nil
}

func fncImageFontWidth(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var font phpv.ZInt
	_, err := core.Expand(ctx, args, &font)
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(gogd.ImageFontWidth(int(font))).ZVal(), nil
}

func fncImageFontHeight(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var font phpv.ZInt
	_, err := core.Expand(ctx, args, &font)
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(gogd.ImageFontHeight(int(font))).ZVal(), nil
}
