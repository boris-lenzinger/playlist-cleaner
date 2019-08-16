package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/eiannone/keyboard"
)

type direction int

const (
	backward direction = iota
	forward
)

var mplayer *exec.Cmd
var stdinMplayer io.WriteCloser
var currentReadVideo string
var toDelete map[string]string
var readingDirection direction

func main() {
	readingDirection = forward
	toDelete := make(map[string]string)
	playlist := os.Args[1]
	haveToFixPlaylist := false
	videosList, err := getContent(playlist)
	if err != nil {
		os.Exit(1)
	}

	err = keyboard.Open()
	if err != nil {
		log.Fatalf("failed to capture stdin: %v\n", err)
	}
	defer keyboard.Close()

	indexInPlaylist := 0
	var currentVideo, nextVideo, prevVideo string
	var waitgroup sync.WaitGroup

	for {
		currentVideo = videosList[indexInPlaylist]
		// Checking if file exists
		if problemInOpeningFile(currentVideo) {
			haveToFixPlaylist = true
			log.Printf("[WARNING] Video %q does not exist. The playlist is not up to date.\n", currentVideo)
			log.Println("[WARNING] The playlist will be fixed in a separated file for future use.")
			if indexInPlaylist == len(videosList)-1 {
				log.Println("[WARNING] You have reached the end of the playlist. Stopping reading.")
				goto endReading
			}
			// have to follow the way the user was reading: forward of backward
			if readingDirection == forward {
				indexInPlaylist++
			} else {
				indexInPlaylist--
			}
			continue
		}

		prevVideo = getPrev(videosList, indexInPlaylist)
		nextVideo = getNext(videosList, indexInPlaylist)
		log.Println(" ================================================== ")
		log.Printf("         Video %d/%d\n", indexInPlaylist+1, len(videosList))
		log.Println(" ================================================== ")
		if _, ok := toDelete[currentVideo]; ok {
			fmt.Println()
			log.Println("       [WARNING] This video has been marked to be deleted")
			fmt.Println()
		}
		go playVideo(currentVideo, &waitgroup)

		log.Println("Available commands: ")
		log.Println("  * > : switch to next video")
		log.Println("  * < : switch to previous video")
		log.Println("  * d : mark video as to be deleted")
		log.Println("  * u : removes video from list to be deleted")
		log.Println("  * q : quit process. Videos marked as deleted will be proposed for deletion")
		log.Println()
		log.Printf("Number of videos marked as to be deleted: %d\n", len(toDelete))

		var s string
		for {
			char, key, err := keyboard.GetKey()
			if err != nil {
				panic(err)
			} else if key == keyboard.KeyEsc {
				break
			}
			s = string(char)
			switch s {
			case ">":
				if nextVideo == "" {
					log.Printf("Already at the end of the playlist. Exiting")
					stopPlayVideo()
					goto endReading
				} else {
					log.Printf("Switching to next file %s\n", nextVideo)
					stopPlayVideo()
					indexInPlaylist++
					readingDirection = forward
					goto playNextVideo
				}
			case "<":
				readingDirection = backward
				if indexInPlaylist > 0 {
					log.Printf("Switching to previous file %s\n", prevVideo)
					indexInPlaylist--
					stopPlayVideo()
					goto playNextVideo
				} else {
					log.Println("Already at the beginning of the playlist. There is no previous file.")
				}
			case "d":
				log.Printf("Requesting to delete %s\n", currentVideo)
				toDelete[currentVideo] = currentVideo
			case "u":
				log.Printf("Requesting to undelete %s\n", currentVideo)
				delete(toDelete, currentVideo)
			case "q":
				log.Printf("Quit reading. Stopped at video %d\n", indexInPlaylist+1)
				stopPlayVideo()
				goto endReading
			default:
				log.Printf("Command %q is unsupported. Please use <,>,d,u,q instead.\n", s)
			}
			log.Printf("[logger] You pressed: %q\r\n", char)
		}
	playNextVideo:

		waitgroup.Wait()
	}
endReading:

	keyboard.Close()

	showDetailsOfDeletion(videosList, toDelete)
	if len(toDelete) > 0 {
		if !confirm() {
			log.Println("Deletion was cancelled")
			return
		}
		deleteSelection(toDelete)
	}

	if haveToFixPlaylist || len(toDelete) > 0 {
		pathNewPlaylist, err := fixPlaylist(playlist)
		if err != nil {
			os.Exit(1)
		}
		log.Printf("Fixed playlist is stored at %q\n", pathNewPlaylist)
	}
}

func getContent(fileContainingList string) ([]string, error) {
	f, err := os.Open(fileContainingList)
	if err != nil {
		log.Printf("Error: failed to open playlist %q due to %s", fileContainingList, err)
		return []string{}, err
	}
	videosListAsBytes, err := ioutil.ReadAll(f)
	if err != nil {
		log.Printf("Error: failed to read playlist %q due to %s", fileContainingList, err)
		return []string{}, err
	}
	videosListAsString := string(videosListAsBytes)
	return strings.Split(videosListAsString, "\n"), nil
}

func problemInOpeningFile(path string) bool {
	_, err := os.Open(path)
	if err != nil {
		return true
	}
	return false
}

func getPrev(list []string, currentIndex int) string {
	if currentIndex > 0 && len(list) > currentIndex {
		return list[currentIndex-1]
	}
	return ""
}

func getNext(list []string, currentIndex int) string {
	if currentIndex < len(list)-2 {
		return list[currentIndex+1]
	}
	return ""
}

func playVideo(pathToVideo string, waitGroup *sync.WaitGroup) {
	waitGroup.Add(1)
	mplayer = exec.Command("/usr/bin/mplayer", pathToVideo)
	mplayer.Start()

	waitGroup.Done()
}

func stopPlayVideo() {
	mplayer.Process.Kill()
}

func sizeToMegaOrGiga(size int64) string {
	sizeInMega := float64(size) / 1024 / 1024
	sizeInGiga := float64(sizeInMega) / 1024

	if sizeInGiga < 1 {
		return fmt.Sprintf("%d Mb", int64(sizeInMega))
	}
	return fmt.Sprintf("%.3f Gb", sizeInGiga)
}

func showDetailsOfDeletion(playlist []string, toDelete map[string]string) {

	if len(toDelete) == 0 {
		log.Println("No file to delete")
		return
	}
	totalSize := int64(0)

	var f *os.File
	var fi os.FileInfo
	for _, path := range playlist {
		if _, ok := toDelete[path]; ok {
			f, _ = os.Open(path)
			fi, _ = f.Stat()
			s := fi.Size()
			fmt.Printf(" * %s %s \n", sizeToMegaOrGiga(s), path)
			totalSize += s
		}
	}
	if totalSize > 0 {
		fmt.Printf("                 Total Size: %s \n", sizeToMegaOrGiga(totalSize))
	}
}

func confirm() bool {
	fmt.Println()
	fmt.Println("Do you confirm deletion ? (Y/N)")
	reader := bufio.NewReader(os.Stdin)

	for {
		answer, _ := reader.ReadString('\n')
		answer = strings.ReplaceAll(answer, "\n", "")
		switch answer {
		case "y", "Y":
			return true
		case "n", "N":
			return false
		default:
			log.Printf("%q is not a supported answer. Please use Y or N\n", answer)
		}
	}
}

func deleteSelection(selection map[string]string) {
	i := 0
	var err error
	for _, path := range selection {
		err = os.Remove(path)
		if err != nil {
			log.Printf("[Warning] Failed to delete %q due to %v\n", path, err)
			continue
		}
		log.Printf("(%d/%d) Deleted %q ...\n", i+1, len(selection), path)
		i++
	}
}

func fixPlaylist(pathToPlaylist string) (string, error) {
	content, err := getContent(pathToPlaylist)
	if err != nil {
		log.Printf("[ERROR] failed to fix playlist %q due to %v\n", pathToPlaylist, err)
		return "", err
	}
	lines := ""
	for _, path := range content {
		if problemInOpeningFile(path) {
			continue
		}
		if lines != "" {
			lines += "\n"
		}
		lines += path
	}
	t := time.Now()
	outputFile := pathToPlaylist + "-" + fmt.Sprintf(t.Format("20060102150405"))
	err = ioutil.WriteFile(outputFile, []byte(lines), 0644)
	if err != nil {
		log.Printf("[ERROR] failed to write fixed playlist to %q due to %v\n", outputFile, err)
		return "", err
	}
	return outputFile, nil
}
