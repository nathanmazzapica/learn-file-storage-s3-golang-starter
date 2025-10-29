package main

import (
	"log"
	"path/filepath"
	"testing"
)

func Test_getVideoAspectRatio(t *testing.T) {
	path := filepath.Join("samples", "boots-video-horizontal.mp4")

	ratio, err := getVideoAspectRatio(path)

	if err != nil {
		log.Println("error:")
		t.Fatal(err)
	}

	if ratio != "16:9" {
		t.Errorf("got %s, want 16:9", ratio)
	}

	path = filepath.Join("samples", "boots-video-vertical.mp4")

	ratio, err = getVideoAspectRatio(path)
	if err != nil {
		log.Println("error:")
		t.Fatal(err)
	}

	if ratio != "9:16" {
		t.Errorf("got %s, want 9:16", ratio)
	}
}
