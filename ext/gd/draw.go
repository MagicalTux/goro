package gd

import (
	"image"

	"github.com/KarpelesLab/gogd"
	"github.com/KarpelesLab/goro/core"
	"github.com/KarpelesLab/goro/core/phpv"
)

func fncImageSetPixel(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var x, y, color phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &x, &y, &color)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageSetPixel(gdImg, int(x), int(y), gogd.Color(color))).ZVal(), nil
}

func fncImageLine(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var x1, y1, x2, y2, color phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &x1, &y1, &x2, &y2, &color)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageLine(gdImg, int(x1), int(y1), int(x2), int(y2), gogd.Color(color))).ZVal(), nil
}

func fncImageRectangle(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var x1, y1, x2, y2, color phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &x1, &y1, &x2, &y2, &color)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageRectangle(gdImg, int(x1), int(y1), int(x2), int(y2), gogd.Color(color))).ZVal(), nil
}

func fncImageFilledRectangle(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var x1, y1, x2, y2, color phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &x1, &y1, &x2, &y2, &color)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageFilledRectangle(gdImg, int(x1), int(y1), int(x2), int(y2), gogd.Color(color))).ZVal(), nil
}

func fncImageEllipse(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var cx, cy, w, h, color phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &cx, &cy, &w, &h, &color)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageEllipse(gdImg, int(cx), int(cy), int(w), int(h), gogd.Color(color))).ZVal(), nil
}

func fncImageFilledEllipse(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var cx, cy, w, h, color phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &cx, &cy, &w, &h, &color)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageFilledEllipse(gdImg, int(cx), int(cy), int(w), int(h), gogd.Color(color))).ZVal(), nil
}

func fncImagePolygon(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var pointsArr *phpv.ZVal
	var color phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &pointsArr, &color)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	points, err := extractPoints(ctx, pointsArr)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImagePolygon(gdImg, points, gogd.Color(color))).ZVal(), nil
}

func fncImageFilledPolygon(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var pointsArr *phpv.ZVal
	var color phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &pointsArr, &color)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	points, err := extractPoints(ctx, pointsArr)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageFilledPolygon(gdImg, points, gogd.Color(color))).ZVal(), nil
}

func fncImageFill(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var x, y, color phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &x, &y, &color)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageFill(gdImg, int(x), int(y), gogd.Color(color))).ZVal(), nil
}

func fncImageSetThickness(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var thickness phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &thickness)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	gogd.ImageSetThickness(gdImg, int(thickness))
	return phpv.ZTrue.ZVal(), nil
}

func fncImageArc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var cx, cy, w, h, start, end, color phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &cx, &cy, &w, &h, &start, &end, &color)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageArc(gdImg, int(cx), int(cy), int(w), int(h), int(start), int(end), gogd.Color(color))).ZVal(), nil
}

func fncImageFilledArc(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var cx, cy, w, h, start, end, color, style phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &cx, &cy, &w, &h, &start, &end, &color, &style)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageFilledArc(gdImg, int(cx), int(cy), int(w), int(h), int(start), int(end), gogd.Color(color), int(style))).ZVal(), nil
}

func fncImageAlphaBlending(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var enable phpv.ZBool
	_, err := core.Expand(ctx, args, &img, &enable)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageAlphaBlending(gdImg, bool(enable))).ZVal(), nil
}

func fncImageSaveAlpha(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var enable phpv.ZBool
	_, err := core.Expand(ctx, args, &img, &enable)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageSaveAlpha(gdImg, bool(enable))).ZVal(), nil
}

func fncImageSetClip(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var x1, y1, x2, y2 phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &x1, &y1, &x2, &y2)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageSetClip(gdImg, int(x1), int(y1), int(x2), int(y2))).ZVal(), nil
}

func fncImageGetClip(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	_, err := core.Expand(ctx, args, &img)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	x1, y1, x2, y2 := gogd.ImageGetClip(gdImg)
	result := phpv.NewZArray()
	result.OffsetSet(ctx, nil, phpv.ZInt(x1).ZVal())
	result.OffsetSet(ctx, nil, phpv.ZInt(y1).ZVal())
	result.OffsetSet(ctx, nil, phpv.ZInt(x2).ZVal())
	result.OffsetSet(ctx, nil, phpv.ZInt(y2).ZVal())
	return result.ZVal(), nil
}

func fncImageColorAt(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var img *phpv.ZVal
	var x, y phpv.ZInt
	_, err := core.Expand(ctx, args, &img, &x, &y)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, img)
	if err != nil {
		return nil, err
	}
	return phpv.ZInt(gogd.ImageColorAt(gdImg, int(x), int(y))).ZVal(), nil
}

// extractPoints converts a PHP array of [x1, y1, x2, y2, ...] to []image.Point.
func extractPoints(ctx phpv.Context, arr *phpv.ZVal) ([]image.Point, error) {
	za := arr.AsArray(ctx)
	if za == nil {
		return nil, nil
	}
	count := int(za.Count(ctx))
	var points []image.Point
	for i := 0; i < count-1; i += 2 {
		xv, _ := za.OffsetGet(ctx, phpv.ZInt(i))
		yv, _ := za.OffsetGet(ctx, phpv.ZInt(i+1))
		x := int(xv.AsInt(ctx))
		y := int(yv.AsInt(ctx))
		points = append(points, image.Point{X: x, Y: y})
	}
	return points, nil
}
