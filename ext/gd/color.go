package gd

import (
	"github.com/KarpelesLab/gogd"
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

func fncImageColorAllocate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var r, g, b phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &r, &g, &b)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	c := gogd.ImageColorAllocate(gdImg, int(r), int(g), int(b))
	return phpv.ZInt(c).ZVal(), nil
}

func fncImageColorAllocateAlpha(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var r, g, b, a phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &r, &g, &b, &a)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	c := gogd.ImageColorAllocateAlpha(gdImg, int(r), int(g), int(b), int(a))
	return phpv.ZInt(c).ZVal(), nil
}

func fncImageColorTransparent(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var color *phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &color)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	if color != nil {
		c := gogd.ImageColorTransparent(gdImg, gogd.Color(*color))
		return phpv.ZInt(c).ZVal(), nil
	}
	// Get current transparent color
	c := gogd.ImageColorTransparent(gdImg, gogd.ColorNone)
	return phpv.ZInt(c).ZVal(), nil
}

func fncImageColorsForIndex(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var color phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &color)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	r, g, b, a := gogd.ImageColorsForIndex(gdImg, gogd.Color(color))
	result := phpv.NewZArray()
	result.OffsetSet(ctx, phpv.ZString("red"), phpv.ZInt(r).ZVal())
	result.OffsetSet(ctx, phpv.ZString("green"), phpv.ZInt(g).ZVal())
	result.OffsetSet(ctx, phpv.ZString("blue"), phpv.ZInt(b).ZVal())
	result.OffsetSet(ctx, phpv.ZString("alpha"), phpv.ZInt(a).ZVal())
	return result.ZVal(), nil
}

func fncImageColorsTotal(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	_, err := core.Expand(ctx, args, &img)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(gogd.ImageColorsTotal(gdImg)).ZVal(), nil
}

func fncImageColorDeallocate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var color phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &color)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	gogd.ImageColorDeallocate(gdImg, gogd.Color(color))
	return phpv.ZTrue.ZVal(), nil
}

func fncImageColorExact(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var r, g, b phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &r, &g, &b)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(gogd.ImageColorExact(gdImg, int(r), int(g), int(b))).ZVal(), nil
}

func fncImageColorClosest(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var r, g, b phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &r, &g, &b)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(gogd.ImageColorClosest(gdImg, int(r), int(g), int(b))).ZVal(), nil
}

func fncImageColorResolve(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var r, g, b phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &r, &g, &b)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(gogd.ImageColorResolve(gdImg, int(r), int(g), int(b))).ZVal(), nil
}
