package main

import (
	"fmt"
	"flag"
	"image"
	"math"
	"os"
	"github.com/disintegration/imaging"
)

func main() {
	err := kriemhild()

	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
}

func kriemhild() error {
	args, err := readargs()

	if err != nil {
		return err
	}

	paths := []string { args.picA, args.picB, }
	imgs, err := readimages(paths)

	if err != nil {
		return err
	}

	err = checkbounds(imgs)

	if err != nil {
		return err
	}

	nrgbas := make([]*image.NRGBA, len(imgs))
	for i, pic := range imgs {
		nrgbas[i] = imaging.Clone(pic)
	}

	from := subimage(nrgbas[1], nrgbas[0])
	from = quodiff(from, float64(args.frames))

	to := subimage(nrgbas[0], nrgbas[1])
	to = quodiff(to, float64(args.frames))

	out := kriemhildtrans(nrgbas[0], nrgbas[1], from, to, args.frames)

	return saveoutput(out)
}

type imagediff struct {
	diff [][]colordiff
}

func (id imagediff) at(x, y int) colordiff {
	return id.diff[x][y]
}

type colordiff struct {
	r float64
	b float64
	g float64
}

type bounds struct {
	xmin int
	ymin int
	xtot int
	ytot int
}

func loopbounds(pic *image.NRGBA) bounds {
	rec := pic.Bounds()

	b := bounds{}

	b.xmin = rec.Min.X;
	b.ymin = rec.Min.Y;
	xmax := rec.Max.X;
	ymax := rec.Max.Y;
	b.xtot = xmax - b.xmin;
	b.ytot = ymax - b.ymin;

	return b
}

func subimage(a, b *image.NRGBA) imagediff {
	bounds := loopbounds(a)

	diff := imagediff{}
	diff.diff = make([][]colordiff, bounds.xtot)
	for i := 0; i < bounds.xtot; i++ {
		diff.diff[i] = make([]colordiff, bounds.ytot)
	}

	x := bounds.xmin
	for i := 0; i < bounds.xtot; i++ {
		y := bounds.ymin
		for j := 0; j < bounds.ytot; j++ {
			cA := a.NRGBAAt(x, y)
			cB := b.NRGBAAt(x, y)

			cd := colordiff{}
			cd.r = float64(cB.R - cA.R)
			cd.g = float64(cB.G - cA.G)
			cd.b = float64(cB.B - cA.B)

			diff.diff[i][j] = cd
			y++
		}
		x++
	}

	return diff
}

func addimagediff(pic *image.NRGBA, diff imagediff) *image.NRGBA {
	bounds := loopbounds(pic)

	sum := imaging.Clone(pic)

	x := bounds.xmin
	for i := 0; i < bounds.xtot; i++ {
		y := bounds.ymin
		for j := 0; j < bounds.ytot; j++ {
			cd := diff.at(x, y)
			col := sum.NRGBAAt(x, y)

			col.R = round(float64(col.R) + cd.r)
			col.G = round(float64(col.G) + cd.g)
			col.B = round(float64(col.B) + cd.b)

			sum.Set(x, y, col)

			y++
		}
		x++
	}

	return sum
}

func subimagediff(pic *image.NRGBA, diff imagediff) *image.NRGBA {
	bounds := loopbounds(pic)

	sum := imaging.Clone(pic)

	x := bounds.xmin
	for i := 0; i < bounds.xtot; i++ {
		y := bounds.ymin
		for j := 0; j < bounds.ytot; j++ {
			cd := diff.at(x, y)
			col := sum.NRGBAAt(x, y)

			col.R = round(float64(col.R) - cd.r)
			col.G = round(float64(col.G) - cd.g)
			col.B = round(float64(col.B) - cd.b)

			sum.Set(x, y, col)

			y++
		}
		x++
	}

	return sum
}

func round(x float64) uint8 {
	if x - math.Floor(x) >= 0.5 {
		return uint8(math.Ceil(x))
	}

	return uint8(math.Floor(x))
}

func quodiff(diff imagediff, div float64) imagediff {
	quo := imagediff{}
	quo.diff = make([][]colordiff, len(diff.diff))

	rowlen := len(diff.diff[0])
	for i, _ := range quo.diff {
		quo.diff[i] = make([]colordiff, rowlen)
	}

	for i, row := range diff.diff {
		for j, cd := range row {
			cd.r = cd.r / div
			cd.g = cd.r / div
			cd.b = cd.r / div

			quo.diff[i][j] = cd
		}
	}

	return quo
}

func checkbounds(imgs []image.Image) error {
	first := imgs[0].Bounds();

	for i := 1; i < len(imgs); i++ {
		if first != imgs[i].Bounds() {
			return fmt.Errorf("Image %v bounds differ", i)
		}
	}

	return nil
}

func readimages(paths []string) ([]image.Image, error) {
	// Well, doing that concurrently was more complex than it seemed.

	picch := make([]chan image.Image, len(paths))

	for i, _ := range picch {
		picch[i] = make(chan image.Image, 1)
	}

	pics := make([]image.Image, len(paths))
	errch := make([]chan error, len(paths))

	for i, _ := range errch {
		errch[i] = make(chan error, 1)
	}

	for i := 0; i < len(paths); i++ {
		go func(path string, ch chan<- image.Image, errch chan<- error) {
			img, err := imaging.Open(path)

			if err != nil {
				errch<- err
				return
			}

			close(errch)
			ch<- img
		}(paths[i], picch[i], errch[i])
	}

	err := filter(errch)

	if err != nil {
		return nil, err
	}

	for i, ch := range picch {
		pics[i] = <-ch
	}

	return pics, nil
}

func saveoutput(out []*image.NRGBA) error {
	errch := make([]chan error, len(out))

	const limit = 10
	sem := make(chan struct{}, limit)

	for i, _ := range errch {
		errch[i] = make(chan error, 1)
	}


	for i, pic := range out {
		go func(i int, pic image.Image, errch chan<- error) {
			sem<- struct{}{}

			defer func() { <-sem }()
			err := writeimage(i, pic)

			if err != nil {
				errch<- err

				return
			}

			errch<- nil
		}(i, pic, errch[i])
	}

	return filter(errch)
}

func filter(errch []chan error) error {
	var catch error

	for _, ech := range errch {
		err, ok := <-ech
		if ok {
			catch = err
		}
	}

	return catch
}

func writeimage(i int, pic image.Image) error {
	filename := fmt.Sprintf("0%v.png", i)

	f, err := os.Create(filename)

	if err != nil {
		return err
	}

	return imaging.Encode(f, pic, imaging.PNG)
}

func kriemhildtrans(picA, picB *image.NRGBA, from, to imagediff, frames int) []*image.NRGBA {
	outlen := frames + 1
	out := make([]*image.NRGBA, outlen)
	out[0] = picA
	out[outlen - 1] = picB

	last := out[0]
	for i := 1; i < len(out) - 1; i++ {
		last = subimagediff(last, from)
		last = addimagediff(last, to)
		out[i] = last
	}

	return out
}

type params struct {
	picA string
	picB string
	frames int
}

func readargs() (*params, error) {
	picA := flag.String("imageA", "", "The first image")
	picB := flag.String("imageB", "", "The second image")
	frames := flag.Uint("frames", 8, "The number of frames between")

	flag.Parse()

	args := &params{}

	args.picA = *picA
	args.picB = *picB
	args.frames = int(*frames)

	if args.picA == "" || args.picB == "" {
		flag.Usage()

		return nil, fmt.Errorf("Missing input path")
	}

	return args, nil
}
