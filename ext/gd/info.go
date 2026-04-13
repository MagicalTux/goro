package gd

import (
	"github.com/KarpelesLab/gogd"
	"github.com/MagicalTux/goro/core"
	"github.com/MagicalTux/goro/core/phpv"
)

func fncGDInfo(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	info := gogd.GDInfo()
	result := phpv.NewZArray()
	for k, v := range info {
		switch val := v.(type) {
		case string:
			result.OffsetSet(ctx, phpv.ZString(k), phpv.ZString(val).ZVal())
		case bool:
			result.OffsetSet(ctx, phpv.ZString(k), phpv.ZBool(val).ZVal())
		}
	}
	return result.ZVal(), nil
}

func fncImageTypes(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	return phpv.ZInt(gogd.ImageTypes()).ZVal(), nil
}

func fncImageResolution(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var imgZ *phpv.ZVal
	var resX, resY *phpv.ZInt
	_, err := core.Expand(ctx, args, &imgZ, &resX, &resY)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, imgZ)
	if err != nil {
		return nil, err
	}
	if resX != nil {
		ry := int(*resX)
		if resY != nil {
			ry = int(*resY)
		}
		gogd.ImageResolution(gdImg, int(*resX), ry)
		return phpv.ZTrue.ZVal(), nil
	}
	rx, ry := gogd.ImageGetResolution(gdImg)
	result := phpv.NewZArray()
	result.OffsetSet(ctx, nil, phpv.ZInt(rx).ZVal())
	result.OffsetSet(ctx, nil, phpv.ZInt(ry).ZVal())
	return result.ZVal(), nil
}

func fncImageFilter(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var imgZ *phpv.ZVal
	var filter phpv.ZInt
	_, err := core.Expand(ctx, args, &imgZ, &filter)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, imgZ)
	if err != nil {
		return nil, err
	}
	var filterArgs []int
	for i := 2; i < len(args); i++ {
		filterArgs = append(filterArgs, int(args[i].AsInt(ctx)))
	}
	return phpv.ZBool(gogd.ImageFilter(gdImg, int(filter), filterArgs...)).ZVal(), nil
}

func fncImageConvolution(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var imgZ *phpv.ZVal
	var matrixArr *phpv.ZVal
	var divisor, offset phpv.ZFloat
	_, err := core.Expand(ctx, args, &imgZ, &matrixArr, &divisor, &offset)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, imgZ)
	if err != nil {
		return nil, err
	}
	// Parse 3x3 matrix from PHP array
	var matrix [3][3]float64
	za := matrixArr.AsArray(ctx)
	if za != nil {
		for i := 0; i < 3; i++ {
			rowVal, _ := za.OffsetGet(ctx, phpv.ZInt(i))
			if rowVal != nil && rowVal.GetType() == phpv.ZtArray {
				row := rowVal.AsArray(ctx)
				for j := 0; j < 3; j++ {
					cellVal, _ := row.OffsetGet(ctx, phpv.ZInt(j))
					if cellVal != nil {
						matrix[i][j] = float64(cellVal.AsFloat(ctx))
					}
				}
			}
		}
	}
	return phpv.ZBool(gogd.ImageConvolution(gdImg, matrix, float64(divisor), float64(offset))).ZVal(), nil
}

func fncImageGammaCorrect(ctx phpv.Context, args []*phpv.ZVal) (*phpv.ZVal, error) {
	var imgZ *phpv.ZVal
	var inputGamma, outputGamma phpv.ZFloat
	_, err := core.Expand(ctx, args, &imgZ, &inputGamma, &outputGamma)
	if err != nil {
		return nil, err
	}
	gdImg, err := getImg(ctx, imgZ)
	if err != nil {
		return nil, err
	}
	return phpv.ZBool(gogd.ImageGammaCorrect(gdImg, float64(inputGamma), float64(outputGamma))).ZVal(), nil
}
