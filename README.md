# bitfountain-download-all

Download all lectures from a [bitfountain](http://bitfountain.io) course!

## Why?

There are times when we do not have access to the internet (e.g. while travelling), and those are also the periods when we have the most amount of spare time :) I believe many of you like spending those times reading, or watching educational videos. There is no easy way to download all the lectures of a [bitfountain](http://bitfountain.io) course at one go, so that we can watch them later. This program, written in [Go lang](http://golang.org), helps you solve this problem.

## How?

Download and unzip the folder.

`cd` into the `bitfountain-download-all` folder and run:

```shell
./bitfountain-download-all -email=your_bitfountain_email_id -pass=your_bitfountain_password -course=bitfountain_course_url
```

e.g.
```shell
./bitfountain-download-all -email=john@example.com -pass=mypass1234 -course=http://bitfountain.io/courses/complete-ios8
```

You can also pass the optional parameter `-section` to specify a starting section for the download. This is zero based, e.g `-section 3` on the `The Bitfountain iOS 7 Immersive` course would be `Challenge 1: Age of Laika`. By default it will download the whole course (starting at section 0).

## Contributions

Othyn

Please create issues and/or submit PRs when you find bugs.
