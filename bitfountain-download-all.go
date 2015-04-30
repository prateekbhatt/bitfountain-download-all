package main

import (
	"flag"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/kardianos/osext"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	// "reflect"
	// "crypto/ssh/terminal"
	// "github.com/tobyhede/go-underscore"
)

func getDashedName(name string, index int) string {

	trimmedName := strings.TrimSpace(name)
	splitNameBySpace := strings.Split(trimmedName, " ")
	dashedName := strings.Join(splitNameBySpace, "-")
	dashedName = fmt.Sprint(strconv.Itoa(index), "-", dashedName)
	return dashedName
}

// func getLatestDownloadedVideo() int {

// }

func main() {

	jar, _ := cookiejar.New(nil)

	LOGIN_URL := "https://sso.usefedora.com/secure/24/users/sign_in"
	SCHOOL_ID := "24" // id of bitfountain on usefedora.com

	// commandline inputs for email, password, and course url
	emailPtr := flag.String("email", "", "Email of the user")
	courseUrlPtr := flag.String("course", "", "URL of the course")
	passwordPtr := flag.String("pass", "", "Password of the user")

	// optionalStartLessonNoPtr := flag.String("no", "", "Lesson no, starts at 0")

	flag.Parse()

	if *emailPtr == "" {
		log.Fatal("Please provide your email address")
	}

	if *passwordPtr == "" {
		log.Fatal("Please provide your password")
	}

	if *courseUrlPtr == "" {
		log.Fatal("Please provide the bitfountain course url")
	}

	// pass, err := terminal.ReadPassword()
	// if err != nil {
	//     log.Fatal(err)
	// }
	// fmt.Printf("\n\npass:: %s", pass)

	fmt.Println("Logging in ...")

	client := http.Client{Jar: jar}

	resp, err := client.PostForm(LOGIN_URL,
		url.Values{
			"user[school_id]": {SCHOOL_ID},
			"user[email]":     {*emailPtr},
			"user[password]":  {*passwordPtr},
		})
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	fmt.Println("Logged in. Fetching course sections ...")

	currentDir, err := osext.ExecutableFolder()
	if err != nil {
		log.Fatal(err)
	}

	courseDir := filepath.Join(currentDir, path.Base(*courseUrlPtr))

	// Every bitfountain course is split into multiple sections with each
	// section having multiple lectures (videos)
	type Lecture struct {
		name     string
		id       string
		filePath string
		url      string // bitfountain lecture page url
	}

	type Section struct {
		name       string
		sectionDir string
		lectures   []Lecture
	}

	// Get the Course page (contains the list of sections and lectures)
	respCourseDetails, err := client.Get(*courseUrlPtr)
	if err != nil {
		log.Fatal(err)
	}
	defer respCourseDetails.Body.Close()

	doc, err := goquery.NewDocumentFromResponse(respCourseDetails)
	if err != nil {
		log.Fatal(err)
	}

	sections := []Section{}

	// Find all the sections in the course, and loop over them
	doc.Find(".course-section").Each(func(i int, s *goquery.Selection) {

		// Get the name of the current section
		name := s.Find(".section-title").Text()

		name = getDashedName(name, i)
		sectionDir := filepath.Join(courseDir, name)

		newLectures := []Lecture{}

		// Find all the lectures in the current section, and loop over them
		s.Find(".section-item").Each(func(i int, l *goquery.Selection) {

			// Get the name of the lecture (video)
			lectureName := l.Find(".lecture-name").Text()

			// Get the lecture id from the attribute. This will be used to
			// construct the url of each lecture's page
			lectureId, _ := l.Attr("data-lecture-id")

			lectureId = strings.TrimSpace(lectureId)

			lectureFileName := fmt.Sprint(getDashedName(lectureName, i), ".mp4")

			// The video will be stored locally at the lectureFilePath
			filePath := filepath.Join(sectionDir, lectureFileName)

			// We will need to visit the lecturePageUrl, to get the Wistia
			// video download link
			url := *courseUrlPtr + "/lectures/" + lectureId

			newLectures = append(newLectures, Lecture{
				name:     lectureName,
				id:       lectureId,
				filePath: filePath,
				url:      url,
			})

		})

		newSection := Section{
			name:       name,
			sectionDir: sectionDir,
			lectures:   newLectures,
		}
		sections = append(sections, newSection)

	})

	// create course directory
	os.Mkdir(courseDir, 0777)

	for _, section := range sections {

		// check if the section dir exists
		_, err := os.Stat(section.sectionDir)
		if err != nil {

			// section dir does not exist
			// create section directory
			os.Mkdir(section.sectionDir, 0777)

		}

		fmt.Printf("\n\n\n%s", section.name)

		for _, lecture := range section.lectures {

			fmt.Printf("\n\n\t%s", lecture.name)

			// Visit the lecture's url
			respLecture, err := client.Get(lecture.url)
			if err != nil {
				log.Fatal(err)
			}

			lecturePage, err := goquery.NewDocumentFromResponse(respLecture)
			if err != nil {
				log.Fatal(err)
			}

			// Get the Wistia download link on the lecture's page
			videoUrl, _ := lecturePage.Find("a.download").Attr("href")

			// Parse the URL and ensure there are no errors.
			parsedVideoUrl, err := url.Parse(videoUrl)
			if err != nil {
				log.Fatal(err)
			}

			// Some of the lectures only have assignments, no video
			// We need to skip them
			if parsedVideoUrl.Host == "" {
				fmt.Printf("\n\t\tNo video found on this lecture's page. Moving on to the next lecture ...")
				continue
			}

			// Sometimes the videoUrl does not have http / https as Scheme,
			// We'll add http as the Scheme, because if its empty Go will
			// throw an error
			if parsedVideoUrl.Scheme != "http" || parsedVideoUrl.Scheme != "https" {
				parsedVideoUrl.Scheme = "http"
				videoUrl = parsedVideoUrl.String()
			}

			var wistiaVideoSize int64

			resp, err := client.Get(videoUrl)
			if err != nil {
				log.Fatal(err)
			} else {
				contentLength := resp.Header.Get("Content-Length")
				wistiaVideoSize, _ = strconv.ParseInt(contentLength, 10, 64)
				// contentType := resp.Header.Get("Content-Type")
				fmt.Printf("\n\t\twistiaVideoSize: %d", wistiaVideoSize)

			}

			// check if video file already exists
			if fileStat, err := os.Stat(lecture.filePath); err == nil {
				existingFileSizeOnDisk := fileStat.Size()
				fmt.Printf("\n\t\texistingFileSizeOnDisk: %d", existingFileSizeOnDisk)

				if existingFileSizeOnDisk == wistiaVideoSize {
					fmt.Println("\n\t\tFull video file exists; moving to next lecture...")
					continue
				} else {
					fmt.Println("\n\t\tVideo file exists but not fully, will download again")
				}
			}

			defer resp.Body.Close()

			fmt.Println("\t\tDownloading video ...")

			out, err := os.Create(lecture.filePath)
			if err != nil {
				log.Fatal(err)
			}
			defer out.Close()

			_, ioErr := io.Copy(out, resp.Body)
			if ioErr != nil {
				log.Fatal(err)
			}

		}
	}
}
