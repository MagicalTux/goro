package gd

import (
	"image"

	"github.com/KarpelesLab/gogd"
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

func fncImageCopy(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var dstZ, srcZ *phpv.ZVal
	var dstX, dstY, srcX, srcY, srcW, srcH phpv.ZInt
	_, err := core.Expand(ctx, args, &dstZ, &srcZ, &dstX, &dstY, &srcX, &srcY, &srcW, &srcH)
	if err != nil {
		return nil, err
	}
	dst, err := getImg(ctx, dstZ)
	if err != nil {
		return nil, err
	}
	src, err := getImg(ctx, srcZ)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageCopy(dst, src, int(dstX), int(dstY), int(srcX), int(srcY), int(srcW), int(srcH))).ZVal(), nil
}

func fncImageCopyResized(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var dstZ, srcZ *phpv.ZVal
	var dstX, dstY, srcX, srcY, dstW, dstH, srcW, srcH phpv.ZInt
	_, err := core.Expand(ctx, args, &dstZ, &srcZ, &dstX, &dstY, &srcX, &srcY, &dstW, &dstH, &srcW, &srcH)
	if err != nil {
		return nil, err
	}
	dst, err := getImg(ctx, dstZ)
	if err != nil {
		return nil, err
	}
	src, err := getImg(ctx, srcZ)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageCopyResized(dst, src, int(dstX), int(dstY), int(srcX), int(srcY), int(dstW), int(dstH), int(srcW), int(srcH))).ZVal(), nil
}

func fncImageCopyResampled(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var dstZ, srcZ *phpv.ZVal
	var dstX, dstY, srcX, srcY, dstW, dstH, srcW, srcH phpv.ZInt
	_, err := core.Expand(ctx, args, &dstZ, &srcZ, &dstX, &dstY, &srcX, &srcY, &dstW, &dstH, &srcW, &srcH)
	if err != nil {
		return nil, err
	}
	dst, err := getImg(ctx, dstZ)
	if err != nil {
		return nil, err
	}
	src, err := getImg(ctx, srcZ)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageCopyResampled(dst, src, int(dstX), int(dstY), int(srcX), int(srcY), int(dstW), int(dstH), int(srcW), int(srcH))).ZVal(), nil
}

func fncImageCopyMerge(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var dstZ, srcZ *phpv.ZVal
	var dstX, dstY, srcX, srcY, srcW, srcH, pct phpv.ZInt
	_, err := core.Expand(ctx, args, &dstZ, &srcZ, &dstX, &dstY, &srcX, &srcY, &srcW, &srcH, &pct)
	if err != nil {
		return nil, err
	}
	dst, err := getImg(ctx, dstZ)
	if err != nil {
		return nil, err
	}
	src, err := getImg(ctx, srcZ)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageCopyMerge(dst, src, int(dstX), int(dstY), int(srcX), int(srcY), int(srcW), int(srcH), int(pct))).ZVal(), nil
}

func fncImageRotate(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var imgZ *phpv.ZVal
	var angle phpv.ZFloat
	var bgColor phpv.ZInt
	_, err := core.Expand(ctx, args, &imgZ, &angle, &bgColor)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, imgZ)
	if err != nil {
		return nil, err
	}
	result := gogd.ImageRotate(gdImg, float64(angle), gogd.Color(bgColor))
	if result == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return wrapImg(ctx, result)
}

func fncImageCrop(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var imgZ *phpv.ZVal
	var rectArr *phpv.ZVal
	_, err := core.Expand(ctx, args, &imgZ, &rectArr)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, imgZ)
	if err != nil {
		return nil, err
	}
	za := rectArr.AsArray(ctx)
	if za == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	xv, _ := za.OffsetGet(ctx, phpv.ZString("x"))
	yv, _ := za.OffsetGet(ctx, phpv.ZString("y"))
	wv, _ := za.OffsetGet(ctx, phpv.ZString("width"))
	hv, _ := za.OffsetGet(ctx, phpv.ZString("height"))
	rect := image.Rect(int(xv.AsInt(ctx)), int(yv.AsInt(ctx)), int(xv.AsInt(ctx))+int(wv.AsInt(ctx)), int(yv.AsInt(ctx))+int(hv.AsInt(ctx)))
	result := gogd.ImageCrop(gdImg, rect)
	if result == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return wrapImg(ctx, result)
}

func fncImageScale(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var imgZ *phpv.ZVal
	var newW phpv.ZInt
	var newH *phpv.ZInt
	var mode *phpv.ZInt
	_, err := core.Expand(ctx, args, &imgZ, &newW, &newH, &mode)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, imgZ)
	if err != nil {
		return nil, err
	}
	h := -1
	if newH != nil {
		h = int(*newH)
	}
	m := 3 // IMG_BILINEAR_FIXED default
	if mode != nil {
		m = int(*mode)
	}
	result := gogd.ImageScale(gdImg, int(newW), h, m)
	if result == nil {
		return phpv.ZFalse.ZVal(), nil
	}
	return wrapImg(ctx, result)
}

func fncImageFlip(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var imgZ *phpv.ZVal
	var mode phpv.ZInt
	_, err := core.Expand(ctx, args, &imgZ, &mode)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, imgZ)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageFlip(gdImg, int(mode))).ZVal(), nil
}
