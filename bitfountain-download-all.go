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

func main() {

	jar, _ := cookiejar.New(nil)

	LOGIN_URL := "https://sso.usefedora.com/secure/24/users/sign_in"
	SCHOOL_ID := "24" // id of bitfountain on usefedora.com

	// commandline inputs for email, password, and course url
	emailPtr := flag.String("email", "", "Email of the user")
	courseUrlPtr := flag.String("course", "", "URL of the course")
	passwordPtr := flag.String("pass", "", "Password of the user")
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

	_, err := client.PostForm(LOGIN_URL,
		url.Values{
			"user[school_id]": {SCHOOL_ID},
			"user[email]":     {*emailPtr},
			"user[password]":  {*passwordPtr},
		})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Logged in. Fetching course sections ...")

	// Get the Course page (contains the list of sections and lectures)
	resp, err := client.Get(*courseUrlPtr)
	if err != nil {
		log.Fatal(err)
	}

	// Every bitfountain course is split into multiple sections with each
	// section having multiple lectures (videos)
	type Lecture struct {
		name      string
		lectureId string
	}

	type Section struct {
		name     string
		lectures []Lecture
	}

	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		log.Fatal(err)
	}

	sections := []Section{}

	// Find all the sections in the course, and loop over them
	doc.Find(".course-section").Each(func(i int, s *goquery.Selection) {

		// Get the name of the current section
		name := s.Find(".section-title").Text()

		newLectures := []Lecture{}

		// Find all the lectures in the current section, and loop over them
		s.Find(".section-item").Each(func(i int, l *goquery.Selection) {

			// Get the name of the lecture (video)
			lectureName := l.Find(".lecture-name").Text()

			// Get the lecture id from the attribute. This will be used to
			// construct the url of each lecture's page
			lectureId, _ := l.Attr("data-lecture-id")
			newLectures = append(newLectures, Lecture{
				name:      lectureName,
				lectureId: lectureId,
			})

		})

		newSection := Section{
			name:     name,
			lectures: newLectures,
		}
		sections = append(sections, newSection)

	})

	currentDir, err := osext.ExecutableFolder()
	if err != nil {
		log.Fatal(err)
	}

	courseDir := filepath.Join(currentDir, path.Base(*courseUrlPtr))

	// create course directory
	os.Mkdir(courseDir, 0777)

	for index, l := range sections {

		sectionDirName := getDashedName(l.name, index)
		sectionDir := filepath.Join(courseDir, sectionDirName)

		// check if the section dir exists
		_, err := os.Stat(sectionDir)
		if err != nil {

			// section dir does not exist
			// create section directory
			os.Mkdir(sectionDir, 0777)

		}

		fmt.Printf("\n\n\n%s", sectionDirName)

		for lIndex, v := range l.lectures {

			lectureId := strings.TrimSpace(v.lectureId)
			lectureName := fmt.Sprint(getDashedName(v.name, lIndex), ".mp4")

			// We will need to visit the lecturePageUrl, to get the Wistia
			// video download link
			lecturePageUrl := *courseUrlPtr + "/lectures/" + lectureId

			// The video will be stored locally at the lectureFilePath
			lectureFilePath := filepath.Join(sectionDir, lectureName)

			fmt.Printf("\n\n\t%s", lectureName)

			// Visit the lecture's url
			respLecture, err := client.Get(lecturePageUrl)
			if err != nil {
				log.Fatal(err)
			}

			lecturePage, err := goquery.NewDocumentFromResponse(respLecture)
			if err != nil {
				log.Fatal(err)
			}

			// Get the Wistia download link on the lecture's page
			videoUrl, _ := lecturePage.Find("a.download").Attr("href")

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
			if fileStat, err := os.Stat(lectureFilePath); err == nil {
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

			out, err := os.Create(lectureFilePath)
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
