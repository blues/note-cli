// Copyright 2023 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package main

import "fmt"

func CobsDecode(input []byte) ([]byte, error) {

	if len(input) == 0 {
		return nil, fmt.Errorf("cobs: no input")
	}

	var output = make([]byte, 0)
	var lastZI = 0

	for {

		if lastZI == len(input) {
			break
		}

		if input[lastZI] == 0x00 {
			return nil, fmt.Errorf("cobs: zero found in input array")
		}

		if int(input[lastZI]) > (len(input) - int(lastZI)) {
			return nil, fmt.Errorf("cobs: out of bounds")
		}

		var nextZI = lastZI + int(input[lastZI])

		for i := lastZI + 1; i < nextZI; i++ {

			if input[i] == 0x00 {
				return nil, fmt.Errorf("cobs: zero not allowed in input")
			}

			output = append(output, input[i])
		}

		if nextZI < len(input) && input[lastZI] != 0xFF {
			output = append(output, 0x00)
		}

		lastZI = nextZI
	}

	return output, nil
}
