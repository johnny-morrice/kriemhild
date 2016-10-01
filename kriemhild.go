package main

import (
	"fmt"
	"flag"
	"image"
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
	pics, err := readimages(paths)

	diff := subimage(pics[0], pics[1])
	fracdiff := quodiff(diff, args.frames)

	out := kriemhildtrans(pics[0], pics[1], fracdiff, args.frames)

	return saveoutput(out)
}

type imagediff struct {
}

func subimage(a, b image.Image) imagediff {
	return imagediff{}
}

func addimagediff(pic image.Image, diff imagediff) image.Image {
	return nil
}

func quodiff(diff imagediff, div int) imagediff {
	return imagediff{}
}

func readimages(paths []string) ([]image.Image, error) {
	// Well, doing that concurrently was more complex than it seemed.

	picch := make([]chan image.Image, len(paths))

	for i, _ := range picch {
		picch[i] = make(chan image.Image)
	}

	pics := make([]image.Image, len(paths))
	errch := make(chan error)

	for i := 0; i < len(paths); i++ {
		go func(path string, ch chan<- image.Image) {
			img, err := imaging.Open(path)

			if err != nil {
				errch<- err
				return
			}

			errch<- nil
			ch<- img
		}(paths[i], picch[i])
	}

	var catch error
	for err := range errch {
		if err != nil {
			catch = err
		}
	}

	if catch != nil {
		return nil, catch
	}

	for i, ch := range picch {
		pics[i] = <-ch
	}

	return pics, nil
}

func saveoutput(out []image.Image) error {
	errch := make(chan error)

	for i, pic := range out {
		go func(i int, pic image.Image) {
			err := writeimage(i, pic)

			if err != nil {
				errch<- err
				return
			}

			errch<- nil
		}(i, pic)
	}

	var catch error
	for err := range errch {
		if err != nil {
			catch = err
		}
	}

	return catch
}

func writeimage(i int, pic image.Image) error {
	filename := fmt.Sprintf("%v.png", i)

	f, err := os.Create(filename)

	if err != nil {
		return err
	}

	return imaging.Encode(f, pic, imaging.PNG)
}

func kriemhildtrans(picA, picB image.Image, diff imagediff, frames int) []image.Image {
	outlen := frames + 1
	out := make([]image.Image, outlen)
	out[0] = picA
	out[outlen - 1] = picB

	last := out[0]
	for i := 1; i < len(out) - 1; i++ {
		last = addimagediff(last, diff)
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

	return args, nil
}
