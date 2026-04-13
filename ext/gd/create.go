package gd

import (
	"github.com/KarpelesLab/gogd"
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

func fncImageCreate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var width, height phpv.ZInt
	_, err := core.Expand(ctx, args, &width, &height)
	if err != nil {
		return nil, err
	}
	img := gogd.ImageCreate(int(width), int(height))
	return wrapImg(ctx, img)
}

func fncImageCreateTrueColor(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var width, height phpv.ZInt
	_, err := core.Expand(ctx, args, &width, &height)
	if err != nil {
		return nil, err
	}
	img := gogd.ImageCreateTrueColor(int(width), int(height))
	return wrapImg(ctx, img)
}

func fncImageDestroy(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	_, err := core.Expand(ctx, args, &img)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	gogd.ImageDestroy(gdImg)
	return phpv.ZTrue.ZVal(), nil
}

func fncImageSX(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	_, err := core.Expand(ctx, args, &img)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(gogd.ImageSX(gdImg)).ZVal(), nil
}

func fncImageSY(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	_, err := core.Expand(ctx, args, &img)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(gogd.ImageSY(gdImg)).ZVal(), nil
}

func fncImageIsTrueColor(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	_, err := core.Expand(ctx, args, &img)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageIsTrueColor(gdImg)).ZVal(), nil
}
