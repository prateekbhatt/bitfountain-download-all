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

	"github.com/pivotal-golang/bytefmt"
	// For Byte formatting on the video size

	"regexp"
	// For filtering out any special characters in the names

	"time"
	"github.com/cheggaaa/pb"
	// For the progress bar
)

func getDashedName(name string, index int) string {

	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		log.Fatal( err )
	}
	safe := reg.ReplaceAllString( name, "-" )
	safe = fmt.Sprint( strconv.Itoa(index), "-", strings.Trim( safe, "-" ) )
	return safe
	// Filters out all special characters
	//	This is to fix the directory error when hitting a lecture name with a forward slash in it
	//	along with any other potential issues with special characters within the filenames
}

// func getLatestDownloadedVideo() int {

// }

func main() {

	jar, _ := cookiejar.New(nil)

	LOGIN_URL := "https://sso.teachable.com/secure/24/users/sign_in?reset_purchase_session=1"
	SCHOOL_ID := "24" // id of bitfountain on usefedora.com

	// commandline inputs for email, password, and course url
	emailPtr	 := flag.String("email", "", "Email of the user")
	courseUrlPtr := flag.String("course", "", "URL of the course")
	passwordPtr	 := flag.String("pass", "", "Password of the user")
	
	optionalStartSectionNoPtr := flag.Int( "section", 0, "Section number to start the downloads at, starts at 0" )
	// Optional start lecture for the download process

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

	fmt.Printf( "\nStarting download at section %d ...\n", *optionalStartSectionNoPtr )
	// Display the starting course for the users reference
	//	Unnecessary, I just like it like that :)

	// pass, err := terminal.ReadPassword()
	// if err != nil {
	//     log.Fatal(err)
	// }
	// fmt.Printf("\n\npass:: %s", pass)

	fmt.Println("Logging in ...")

	client := http.Client{Jar: jar}

	respLogin, err := client.Get(LOGIN_URL)
	if err != nil {
		log.Fatal(err)
	}

	loginPage, err := goquery.NewDocumentFromResponse(respLogin)
	if err != nil {
		log.Fatal(err)
	}
	defer respLogin.Body.Close()

	token, _ := loginPage.Find("input[name='authenticity_token']").Attr("value")

	resp, err := client.PostForm(LOGIN_URL,
		url.Values{
			"utf8": {"&#x2713;"},
			"authenticity_token": {token},
			"user[school_id]": {SCHOOL_ID},
			"user[email]":     {*emailPtr},
			"user[password]":  {*passwordPtr},
			"commit": {"Log in"},
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

		if i >= *optionalStartSectionNoPtr {

			// Only add the section if it is above the requested section index

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
				item, _ := l.Find(".item").Attr("href")
				parts := strings.Split(item, "/")
				lectureId := strings.TrimSpace(parts[len(parts)-1])

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

		}
		// This should provide rudamentary functionality to the optionalStartSectionNoPtr param

	})

	fmt.Println( "Done! Starting course download..." )
	// Just providing the user with more feedback

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
			defer respLecture.Body.Close()

			// Get the Wistia download link on the lecture's page
			videoUrl, _ := lecturePage.Find("a.download").Attr("href")

			// Parse the URL and ensure there are no errors.
			parsedVideoUrl, err := url.Parse(videoUrl)
			if err != nil {
				log.Fatal(err)
			}

			// Sometimes the videoUrl does not have http / https as Scheme,
			// We'll add http as the Scheme, because if its empty Go will
			// throw an error
			if parsedVideoUrl.Scheme != "http" || parsedVideoUrl.Scheme != "https" {
				parsedVideoUrl.Scheme = "http"
				videoUrl = parsedVideoUrl.String()
				parsedVideoUrl, _ = url.Parse(videoUrl)

			}

			// Some of the lectures only have assignments, no video
			// We need to skip them
			if parsedVideoUrl.Host == "" {
				fmt.Printf("\n\t\tNo video found on this lecture's page. Moving on to the next lecture ...")
				continue
			}

			var wistiaVideoSize int64

			var wistiaVideoSizeString string
			var existingFileSizeOnDiskString string
			// Create some new variables for the converted byte strings
			//	It's all for readability, instead of placing the converted string straight into the printf

			resp, err := client.Get(videoUrl)
			if err != nil {
				log.Fatal(err)
			} else {
				contentLength := resp.Header.Get("Content-Length")
				wistiaVideoSize, _ = strconv.ParseInt(contentLength, 10, 64)
				// contentType := resp.Header.Get("Content-Type")

				wistiaVideoSizeString = bytefmt.ByteSize( uint64( wistiaVideoSize ) )
				// Convert the video size to a human readable format and place it in a new var

				fmt.Printf("\n\t\twistiaVideoSize: %s", wistiaVideoSizeString)

			}

			// check if video file already exists
			if fileStat, err := os.Stat(lecture.filePath); err == nil {
				existingFileSizeOnDisk := fileStat.Size()

				existingFileSizeOnDiskString = bytefmt.ByteSize( uint64( existingFileSizeOnDisk ) )
				// Convert the video size to a human readable format and place it in a new var

				fmt.Printf("\n\t\texistingFileSizeOnDisk: %s", existingFileSizeOnDiskString)

				if existingFileSizeOnDisk == wistiaVideoSize {
					fmt.Println("\n\t\tFull video file exists; moving to next lecture...")
					continue
				} else {
					fmt.Printf("\n\t\tVideo file exists but not fully, will download again")
				}
			}

			defer resp.Body.Close()

			fmt.Printf("\n\t\tCreating file ...")

			out, err := os.Create(lecture.filePath)
			if err != nil {
				log.Fatal(err)
			}
			defer out.Close()

			fmt.Printf(" Done!\n\t\tDownloading Video ... \n")

			// Create the progress bar
			bar := pb.New( int( wistiaVideoSize ) ).SetUnits( pb.U_BYTES ).SetRefreshRate( time.Second / 2 )
			bar.ShowSpeed = true
			bar.SetMaxWidth(80)
			bar.Format("[=> ]")
			// Custom init options

			bar.Callback = func(progressBarString string) {
				fmt.Printf("\r\t\t%v", progressBarString)
			}
			// Custom formatting callback, makes the progress bar stay on the same line, but have tabbing to keep inline with the other elements

			bar.Start()
			// Start the progress bar

			writer := io.MultiWriter(out, bar)
			// Create multi writer

			_, ioErr := io.Copy(writer, resp.Body)
			if ioErr != nil {
				log.Fatal(err)
			}
			// Copy (download) the file

			bar.Finish()
			// Stop the progress bar

			fmt.Printf("\t\tDone!")
		}
	}
}