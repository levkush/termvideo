package main

import (
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/color"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	vidio "github.com/AlexEidt/Vidio"
	aio "github.com/AlexEidt/aio"
)

var FILE_NAME = "BLANK.mp4"
var QUALITY = 8
var FPS_MODIFIER = 35
var DELETE_AFTER_FINISHING = false
var timed = false
var last_run_time = time.Now()
var CALIBRATED = false
var CALIBRATE_TIME = time.Now()

func moveCursorUp(lines int) {
	fmt.Printf("\033[%dA", lines)
}

func playAudio(audio *aio.Audio, player *aio.Player) {
	go func() {
		player.Play(audio.Buffer())
	}()
}

func processFrame(img *image.RGBA, qualityX, qualityY int) string {
	bounds := img.Bounds()
	width, height := bounds.Max.X, bounds.Max.Y

	var builder strings.Builder

	for y := 0; y < height; y += qualityY {
		for x := 0; x < width; x += qualityX {
			// Get the RGBA value of the pixel directly without shifting
			r, g, b, _ := img.At(x, y).RGBA()

			// Create an ANSI escape code for colored text
			builder.WriteString(fmt.Sprintf("\x1b[38;2;%d;%d;%dm\x1b[48;2;%d;%d;%dm ", r>>8, g>>8, b>>8, r>>8, g>>8, b>>8))
		}
		builder.WriteString("\n")
	}

	return builder.String()
}

func downloadFile(filepath string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

func createBuffer(video vidio.Video, qualityX, qualityY int) {
	fps := int(video.FPS())
	img := image.NewRGBA(image.Rect(0, 0, video.Width(), video.Height()))
	video.SetFrameBuffer(img.Pix)

	// playAudio("synctest.mp4")

	audio, _ := aio.NewAudio(FILE_NAME, nil)
	player, _ := aio.NewPlayer(audio.Channels(), audio.SampleRate(), audio.Format())
	defer player.Close()

	frame_counter := fps

	for video.Read() {
		if frame_counter == fps {
			audio.Read()

			if time.Since(CALIBRATE_TIME).Truncate(time.Duration(100)*time.Millisecond) > time.Duration(30)*time.Second {
				CALIBRATED = false
			}

			if timed && !CALIBRATED {
				cycle_time := time.Since(last_run_time).Truncate(time.Duration(100) * time.Millisecond)

				if cycle_time < time.Duration(1000)*time.Millisecond {
					FPS_MODIFIER--
				}

				if cycle_time > time.Duration(1000)*time.Millisecond {
					FPS_MODIFIER++
				}

				if cycle_time == time.Duration(1000)*time.Millisecond {
					FPS_MODIFIER++

					CALIBRATED = true
					CALIBRATE_TIME = time.Now()
				}
			}

			last_run_time = time.Now()

			if !timed {
				timed = true
			}

			playAudio(audio, player)
			frame_counter = 0
		}
		frame := processFrame(img, qualityX, qualityY)
		fmt.Print(frame)
		moveCursorUp(strings.Count(frame, "\n"))

		time.Sleep(time.Second / time.Duration(fps+FPS_MODIFIER))
		//time.Sleep(time.Second / time.Duration(fps+70))

		frame_counter++

		//fmt.Println(frame_counter)
	}
}

func clearScreen() {
	cmd := exec.Command("cmd", "/c", "cls")
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func main() {
	clearScreen()
	if _, err := os.Stat("cache.mp4"); err == nil {
		os.Remove("cache.mp4")
	}

	help := false

	flag.BoolVar(&help, "h", false, "Show this message")
	flag.IntVar(&QUALITY, "q", 8, "Quality scaling modifier, lower is better")

	flag.Parse()

	if help {
		fmt.Println("USAGE: termvideo (flags) [filename/url]")
		flag.PrintDefaults()
		return
	}

	args := flag.Args()

	if len(args) <= 0 {
		fmt.Println("USAGE: termvideo (flags) [filename/url]")
		flag.PrintDefaults()
		return
	}

	FILE_NAME = args[0]

	if strings.HasPrefix(FILE_NAME, "http") {
		if runtime.GOOS == "windows" {
			if _, err := os.Stat("youtube-dl.exe"); errors.Is(err, os.ErrNotExist) {
				downloadFile("youtube-dl.exe", "https://github.com/ytdl-org/ytdl-nightly/releases/latest/download/youtube-dl.exe")
			}
		} else {
			if _, err := os.Stat("youtube-dl"); errors.Is(err, os.ErrNotExist) {
				downloadFile("youtube-dl", "https://github.com/ytdl-org/ytdl-nightly/releases/latest/download/youtube-dl")
			}
		}
		if runtime.GOOS == "windows" {
			cmd := exec.Command("./youtube-dl.exe", "-q", "-f", "mp4[height=360]", "-o", "cache.mp4", FILE_NAME)
			cmd.Stdout = os.Stdout
			cmd.Run()
		} else {
			cmd := exec.Command("./youtube-dl", "-q", "-f", "mp4[height=360]", "-o", "cache.mp4", FILE_NAME)
			cmd.Stdout = os.Stdout
			cmd.Run()
		}

		FILE_NAME = "cache.mp4"

		DELETE_AFTER_FINISHING = true
	}

	if _, err := os.Stat(FILE_NAME); errors.Is(err, os.ErrNotExist) {
		fmt.Println("USAGE: termvideo (flags) [filename/url]")
		flag.PrintDefaults()
		return
	}

	quality := QUALITY
	qualityX := quality / 2
	qualityY := quality

	video, _ := vidio.NewVideo(FILE_NAME)

	createBuffer(*video, qualityX, qualityY)

	clearScreen()

	if DELETE_AFTER_FINISHING {
		os.Remove("cache.mp4")
	}
}
