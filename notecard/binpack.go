// Copyright 2017 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import (
	"archive/zip"
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
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

// For nrf52 DFU the region indicates the use of the binary data in each binpack LOAD section
const nrfRegionJSONManifest int = 1 // JSON manifest from .zip DFU file
const nrfRegionMetadata int = 2     // .dat metadata from .zip DFU file
const nrfRegionBinary int = 3       // Binary from .zip DFU file containing application, bootloader & softdevice executable
const nrfRegionQSPIFlash int = 4    // Circuit python disc image for nRF external QSPI flash

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
	firmwareinfo := ""
	for _, pair := range args {

		// Parse the arg
		var fnArg string
		var loadAddrArg string
		var addressArg int
		var regionArg int
		pairSplit := strings.Split(pair, ":")
		if len(pairSplit) == 1 {
			loadAddrArg = "0"
			fnArg = pairSplit[0]

			// nRF52 Cct Python disc image are assigned region 4 to tell the Notecard to
			// use the external QSPI flash programming commands.  Note that keying from the
			// file extension here is just to simplify the binpack process for customers who
			// generate the cct python disc image using Blues tools.
			if strings.HasPrefix(hostProcessorType, "nrf") && strings.HasSuffix(fnArg, "cpy") {
				regionArg = nrfRegionQSPIFlash
			}
		} else if pairSplit[0] == "" || pairSplit[1] == "" {
			return badFmtErr
		} else {
			loadAddrArg = pairSplit[0]
			fnArg = pairSplit[1]
			numsplit := strings.Split(loadAddrArg, ",")
			if len(numsplit) == 1 {
				addressArg, err = parseNumber(loadAddrArg)
				if err != nil {
					return err
				}
				regionArg = -1
			} else {
				addressArg, err = parseNumber(numsplit[0])
				if err != nil {
					return err
				}
				regionArg, err = parseNumber(numsplit[1])
				if err != nil {
					return err
				}
			}
		}

		// Form an actual file path
		if strings.HasPrefix(fnArg, "~/") {
			usr, _ := user.Current()
			fnArg = filepath.Join(usr.HomeDir, fnArg[2:])
		}

		// Handle ZIP files
		fnArray := []string{}
		binArray := [][]byte{}
		addressArray := []int{}
		regionArray := []int{}
		if !strings.HasSuffix(fnArg, ".zip") {
			fnArray = append(fnArray, filepath.Base(fnArg))
			bin, err := os.ReadFile(fnArg)
			if err != nil {
				return fmt.Errorf("%s: %s", fnArg, err)
			}
			binArray = append(binArray, bin)
			addressArray = append(addressArray, addressArg)
			regionArray = append(regionArray, regionArg)
		} else {
			addressArray, regionArray, fnArray, binArray, err = readZip(hostProcessorType, fnArg)
			if err != nil {
				return fmt.Errorf("%s: %s", fnArg, err)
			}
		}

		// Loop, appending the files
		for i := range fnArray {
			fn := fnArray[i]
			bin := binArray[i]
			address := addressArray[i]
			region := regionArray[i]
			if firmwareinfo == "" {
				firmwareinfo = extractLine(&bin, "firmware::info:")
			}

			// Append to the lists
			filenames = append(filenames, fn)
			files = append(files, bin)
			addresses = append(addresses, address)
			if region == -1 {
				region = len(bin)
			}
			regions = append(regions, region)

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
	if firmwareinfo != "" {
		prefix += "INFO: " + firmwareinfo + "\n"
		hprefix += "INFO: " + firmwareinfo + "\n"
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

// Read a nordic ZIP file
func readZip(hostProcessorType string, path string) (addressArray []int, regionArray []int,
	filenameArray []string, binArray [][]byte, err error) {

	// If this isn't a nordic zip file, it's not supported
	if !strings.HasPrefix(hostProcessorType, "nrf") {
		err = fmt.Errorf("only nordic zip files supported")
		return
	}

	// Read the ZIP contents
	var zipContents []byte
	zipContents, err = os.ReadFile(path)
	if err != nil {
		return
	}

	// Prepare to sort through the files in the ZIP
	namesJSON := []string{}
	filesJSON := [][]byte{}
	namesDAT := []string{}
	filesDAT := [][]byte{}
	namesBIN := []string{}
	filesBIN := [][]byte{}
	namesOTHER := []string{}
	filesOTHER := [][]byte{}

	// Unzip the files within the zip
	archive, err2 := zip.NewReader(bytes.NewReader(zipContents), int64(len(zipContents)))
	if err2 != nil {
		err = err2
		return
	}
	for _, zf := range archive.File {
		f, err2 := zf.Open()
		if err2 != nil {
			err = err2
			return
		}
		contents, err2 := io.ReadAll(f)
		f.Close()
		if err2 != nil {
			err = err2
			return
		}
		if strings.HasSuffix(zf.Name, ".json") {
			namesJSON = append(namesJSON, zf.Name)
			filesJSON = append(filesJSON, contents)
		} else if strings.HasSuffix(zf.Name, ".dat") {
			namesDAT = append(namesDAT, zf.Name)
			filesDAT = append(filesDAT, contents)
		} else if strings.HasSuffix(zf.Name, ".bin") {
			namesBIN = append(namesBIN, zf.Name)
			filesBIN = append(filesBIN, contents)
		} else {
			namesOTHER = append(namesOTHER, zf.Name)
			filesOTHER = append(filesOTHER, contents)
		}
	}

	// Append to results

	// JSON manifest content from .zip DFU file
	for i := range namesJSON {
		addressArray = append(addressArray, 0)
		regionArray = append(regionArray, nrfRegionJSONManifest)
		filenameArray = append(filenameArray, namesJSON[i])
		binArray = append(binArray, filesJSON[i])
	}

	// Metadata content from .zip DFU file
	for i := range namesDAT {
		addressArray = append(addressArray, 0)
		regionArray = append(regionArray, nrfRegionMetadata)
		filenameArray = append(filenameArray, namesDAT[i])
		binArray = append(binArray, filesDAT[i])
	}

	// Binary data from .zip DFU file containing application, bootloader & softdevice executable
	for i := range namesBIN {
		addressArray = append(addressArray, 0)
		regionArray = append(regionArray, nrfRegionBinary)
		filenameArray = append(filenameArray, namesBIN[i])
		binArray = append(binArray, filesBIN[i])
	}

	for i := range namesOTHER {
		addressArray = append(addressArray, 0)
		regionArray = append(regionArray, 0) // Region: 0 for anything else
		filenameArray = append(filenameArray, namesOTHER[i])
		binArray = append(binArray, filesOTHER[i])
	}

	// If no files, error
	if len(filenameArray) == 0 {
		err = fmt.Errorf("no files in ZIP")
		return
	}

	// Done
	return

}

// extractLine explores the contents of a bin to find a string with the specified prefix
func extractLine(payloadOriginal *[]byte, contains string) (found string) {
	payload := *payloadOriginal
	components := bytes.SplitAfterN(payload, []byte(contains), 2)
	if len(components) > 1 {
		length := bytes.IndexRune(components[1], 0)
		if length != -1 {
			str := components[1][0:length]
			str = bytes.TrimRight(str, "\r\n")
			found = contains + string(str)
		}
	}
	return
}
