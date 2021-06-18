package main

import (
	"testing"
)

var profile0 = `
	{
	   "GlobalOpts": {
	     "aligned": "keyframe",
	     "livesize": "0"
	   },
		"OutputsOpts" : [
		{
		   "StreamID": "720",
		   "VideoPID": 256
		},
		{
		   "StreamID": "Audio0",
		   "AudioPID": 257
		},
		{
		   "StreamID": "Audio1",
		   "AudioPID": 258
		}
	   ]
	 }
`

func TestMpeg(t *testing.T) {

}
