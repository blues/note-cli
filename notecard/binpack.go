// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/golang/snappy"
)

// Chunk size of uncompressed frames within the binpack, which determines how much
// memory is used when unpacking.
const uncompressedFrameMax = 8192

// Collects multiple .bin files into a single multi-bin file for composite sideloads/downloads
func dfuPackage(verbose bool, outfile string, hostProcessorType string, args []string) (err error) {

	// Preset error
	badFmtErr := fmt.Errorf("MCU type must be followed addr:bin list such as '0x0:bootloader.bin 0x10000:user.bin'")

	// Parse args
	if len(args) == 0 {
		return badFmtErr
	}

	// Read the contents of the files
	addresses := []int{}
	regions := []int{}
	filenames := []string{}
	files := [][]byte{}
	for _, pair := range args {

		pairSplit := strings.Split(pair, ":")
		if len(pairSplit) < 2 || pairSplit[0] == "" || pairSplit[1] == "" {
			return badFmtErr
		}

		fn := pairSplit[1]
		filenames = append(filenames, filepath.Base(fn))

		if strings.HasPrefix(fn, "~/") {
			usr, _ := user.Current()
			fn = filepath.Join(usr.HomeDir, fn[2:])
		}
		bin, err := ioutil.ReadFile(fn)
		if err != nil {
			return fmt.Errorf("%s: %s", fn, err)
		}

		// Round the length to a multiple of 4 bytes by
		// padding it with zero's
		if len(bin)%4 != 0 {
			padBytes := 4 - (len(bin) % 4)
			buf := make([]byte, padBytes)
			bin = append(bin, buf...)
		}

		// Append the the list of files
		files = append(files, bin)

		var num int
		numstr := pairSplit[0]
		numsplit := strings.Split(numstr, ",")
		if len(numsplit) == 1 {
			num, err = parseNumber(numstr)
			if err != nil {
				return err
			}
			addresses = append(addresses, num)
			regions = append(regions, len(bin))
		} else {
			num, err = parseNumber(numsplit[0])
			if err != nil {
				return err
			}
			addresses = append(addresses, num)
			num, err = parseNumber(numsplit[1])
			if err != nil {
				return err
			}
			regions = append(regions, num)
		}

	}

	// Generate the compressed form of the binaries
	// Concatenate the binaries
	compressionSavings := 0
	filesCompressed := [][]byte{}
	for i := range files {

		// Emit snappy-compressed frames
		uncompressedBin := files[i]
		uncompressedBinOffset := 0
		uncompressedBinLeft := len(uncompressedBin)
		compressedBin := []byte{}
		for uncompressedBinLeft > 0 {

			uncompressedFrameLen := uncompressedFrameMax
			if uncompressedBinLeft < uncompressedFrameMax {
				uncompressedFrameLen = uncompressedBinLeft
			}
			uncompressedFrame := uncompressedBin[uncompressedBinOffset : uncompressedBinOffset+uncompressedFrameLen]
			compressedFrame := snappy.Encode(nil, uncompressedFrame)
			compressedFrameLen := len(compressedFrame)

			if compressedFrameLen >= uncompressedFrameLen {
				frameHeader := []byte{
					byte(uncompressedFrameLen & 0xff),
					byte((uncompressedFrameLen >> 8) & 0xff),
					byte((uncompressedFrameLen >> 16) & 0xff),
					byte((uncompressedFrameLen>>24)&0x7f) | 0x80}
				compressedBin = append(compressedBin, frameHeader...)
				compressedBin = append(compressedBin, uncompressedFrame...)
				if verbose {
					fmt.Printf("binpack %s frame len %d (uncompressed)\n", filenames[i], uncompressedFrameLen)
				}
			} else {
				frameHeader := []byte{
					byte(compressedFrameLen & 0xff),
					byte((compressedFrameLen >> 8) & 0xff),
					byte((compressedFrameLen >> 16) & 0xff),
					byte((compressedFrameLen >> 24) & 0x7f)}
				compressedBin = append(compressedBin, frameHeader...)
				compressedBin = append(compressedBin, compressedFrame...)
				compressionSavings += uncompressedFrameLen - compressedFrameLen
				if verbose {
					fmt.Printf("binpack %s frame len %d -> %d\n", filenames[i], uncompressedFrameLen, compressedFrameLen)
				}
			}

			compressionSavings -= 4 // frame header

			uncompressedBinLeft -= uncompressedFrameLen
			uncompressedBinOffset += uncompressedFrameLen

		}

		filesCompressed = append(filesCompressed, compressedBin)

	}

	// Build the prefix string
	now := time.Now().UTC()
	prefix := "/// BINPACK ///\n"
	prefix += "WHEN: " + now.Format("2006-01-02 15:04:05 UTC") + "\n"
	line := "HOST: " + hostProcessorType + "\n"
	prefix += line
	hprefix := line
	prefix += fmt.Sprintf("SNAP: %d\n", uncompressedFrameMax)
	for i := range addresses {
		cleanFn := strings.ReplaceAll(filenames[i], ",", "")
		prefix += fmt.Sprintf("LOAD: %s,%d,%d,%d,%d,%x\n", cleanFn, addresses[i], regions[i], len(files[i]), len(filesCompressed[i]), md5.Sum(files[i]))
		hprefix += fmt.Sprintf("LOAD: %s,0x%08x,0x%x,0x%x\n", cleanFn, addresses[i], regions[i], len(files[i]))
	}
	prefix += "/// BINPACK ///\n"

	// Create the output file
	ext := ".binpack"
	if strings.HasPrefix(outfile, "~/") {
		usr, _ := user.Current()
		outfile = filepath.Join(usr.HomeDir, outfile[2:])
	}
	if outfile == "" {
		outfile = now.Format("2006-01-02-150405") + ext
	} else if strings.HasSuffix(outfile, "/") {
		outfile += now.Format("2006-01-02-150405") + ext
	} else if !strings.HasSuffix(outfile, ext) {
		tmp := strings.Split(outfile, ".")
		if len(tmp) > 1 {
			outfile = strings.Join(tmp[:len(tmp)-1], ".")
		}
		outfile += ext
	}

	os.Remove(outfile)
	fd, err := os.Create(outfile)
	if err != nil {
		return err
	}

	// Write the prefix, with its terminators
	fd.Write([]byte(prefix))
	fd.Write([]byte{0})

	// Write the compressed binary
	for i := range filesCompressed {
		fd.Write(filesCompressed[i])
	}

	// Don't need file anymore
	fd.Close()

	// Get stats
	fi, err := os.Stat(outfile)
	if err != nil {
		return err
	}

	// Done
	fmt.Printf("%s now incorporates %d files and is %d bytes (%d%% saved because of compression):\n\n%s\n", outfile, len(addresses), fi.Size(), int64(compressionSavings*100)/fi.Size(), hprefix)
	return nil

}

// Parse a number, allowing for hex or decimal
func parseNumber(numstr string) (num int, err error) {
	var num64 int64
	if strings.HasPrefix(numstr, "0x") || strings.HasPrefix(numstr, "0X") {
		numstr = strings.TrimPrefix(strings.TrimPrefix(numstr, "0x"), "0X")
		num64, err = strconv.ParseInt(numstr, 16, 64)
		if err != nil {
			return 0, err
		}
		return int(num64), nil
	}
	num64, err = strconv.ParseInt(numstr, 10, 64)
	if err != nil {
		return 0, err
	}
	return int(num64), nil
}

// Determines whether or not a firmware image is a Notecard image or a User image
func dfuIsNotecardFirmware(bin *[]byte) (isNotecardImage bool) {

	// NotecardFirmwareSignature is used to identify whether or not this firmware is a
	// candidate for downloading onto notecards.  Note that this is not a security feature; if someone
	// embeds this binary sequence and embeds it, they will be able to do precisely what they can do
	// by using the USB to directly download firmware onto the device. This mechanism is intended for
	// convenience and is just intended to keep people from inadvertently hurting themselves.
	var NotecardFirmwareSignature = []byte{0x82, 0x1c, 0x6e, 0xb7, 0x18, 0xec, 0x4e, 0x6f, 0xb3, 0x9e, 0xc1, 0xe9, 0x8f, 0x22, 0xe9, 0xf6}

	return bytes.Contains(*bin, NotecardFirmwareSignature)

}
