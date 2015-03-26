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

	// options := cookiejar.Options{
	//     PublicSuffixList: publicsuffix.List,
	// }
	jar, _ := cookiejar.New(nil)
	// if err != nil {
	//     log.Fatal(err)
	// }

	LOGIN_URL := "https://sso.usefedora.com/secure/24/users/sign_in"
	SCHOOL_ID := "24" // id of bitfountain on usefedora.com

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

	resp, err := client.Get(*courseUrlPtr)
	if err != nil {
		log.Fatal(err)
	}

	// fmt.Printf("\n\nrespppp:: %s", resp)
	// cookies := loginresp.Cookies()
	// fmt.Printf("\n\nresp:: %s", cookies[0])

	// iteratorFn := func(c, i interface{}) {
	//     fmt.Printf("\n\ncookie:: ", c.String())
	//     fmt.Printf("\ncookie interface:: ", i)
	// }

	// un.Each(iteratorFn, cookies)

	type lecture struct {
		name      string
		lectureId string
	}

	type section struct {
		name     string
		lectures []lecture
	}

	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		log.Fatal(err)
	}

	sections := []section{}

	doc.Find(".course-section").Each(func(i int, s *goquery.Selection) {
		name := s.Find(".section-title").Text()
		newLectures := []lecture{}

		s.Find(".section-item").Each(func(i int, l *goquery.Selection) {
			lectureName := l.Find(".lecture-name").Text()
			lectureId, _ := l.Attr("data-lecture-id")
			newLectures = append(newLectures, lecture{
				name:      lectureName,
				lectureId: lectureId,
			})

		})

		newLesson := section{
			name:     name,
			lectures: newLectures,
		}
		sections = append(sections, newLesson)

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

			lecturePageUrl := *courseUrlPtr + "/lectures/" + lectureId

			lectureFilePath := filepath.Join(sectionDir, lectureName)

			fmt.Printf("\n\n\t%s", lectureName)

			if _, err := os.Stat(lectureFilePath); err == nil {
				fmt.Printf("\n\tVideo file exists; moving to next lecture...")
				continue
			}

			respLecture, err := client.Get(lecturePageUrl)
			if err != nil {
				log.Fatal(err)
			}

			lecturePage, err := goquery.NewDocumentFromResponse(respLecture)
			if err != nil {
				log.Fatal(err)
			}

			videoUrl, _ := lecturePage.Find("a.download").Attr("href")

			fmt.Printf("\n\tDownloading video ...")

			out, err := os.Create(lectureFilePath)
			if err != nil {
				log.Fatal(err)
			}
			defer out.Close()

			resp, err := client.Get(videoUrl)
			if err != nil {
				log.Fatal(err)
			}
			defer resp.Body.Close()

			_, ioErr := io.Copy(out, resp.Body)
			if ioErr != nil {
				log.Fatal(err)
			}

		}
	}
}
