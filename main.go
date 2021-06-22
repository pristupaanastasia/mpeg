package main

import (
	"bytes"
	"fmt"
	"github.com/Comcast/gots/packet"
	"github.com/Comcast/gots/packet/adaptationfield"
	"github.com/Comcast/gots/pes"
	"github.com/Comcast/gots/psi"
	"github.com/pkg/errors"
	"io"
	"log"
	"os"
	"strconv"
)

var Cc map[int]int
var PTS map[int]uint64

func MpegTeg(r io.Reader, M3u8 io.Writer) error {
	Cc = make(map[int]int, 0)
	PTS = make(map[int]uint64, 0)
	pktData := make([]byte, packet.PacketSize)

	var pat psi.PAT
	PUSI := make(map[int]pes.PESHeader)
	PES := make(map[int]*packet.Packet)
	pmtPid := 0
	var PMT psi.PMT

	var i int
	var name int
	var video int
	io.WriteString(M3u8, "#EXTM3U\n")
	io.WriteString(M3u8, "#EXT-X-TARGETDURATION:10\n")
	io.WriteString(M3u8, "#EXT-X-MEDIA-SEQUENCE:1\n")
	File, err := os.Create(strconv.Itoa(name) + ".ts")
	if err != nil {
		log.Fatal(err)
	}

	for {
		_, err := io.ReadFull(r, pktData)
		if err != nil {
			if err == io.EOF {
				io.WriteString(M3u8, "#EXT-X-ENDLIST\n")
				log.Println(err)
				return err
			}
			return errors.Wrap(err, "cant read header")
		}
		pkt, err := packet.FromBytes(pktData)
		if err != nil {
			log.Println(err)
			return errors.WithMessage(err, "create packet from bytes")
		}

		if (pkt.ContinuityCounter()-1 != Cc[pkt.PID()] && pkt.ContinuityCounter() != 0) || (pkt.ContinuityCounter() == 0 && Cc[pkt.PID()] != 15) {
			//todo здесь будет вставка ошибки в hls - сс пропущен
			log.Println("ошибка Сс")
			io.WriteString(M3u8, "#EXT-X-DISCONTINUITY\n")
		}
		Cc[pkt.PID()] = pkt.ContinuityCounter()

		if packet.IsPat(pkt) {
			pat, err = psi.ReadPAT(bytes.NewReader(pkt[:]))
			if err != nil {
				log.Println("pat", err)
				return errors.WithMessage(err, "PAT")
			}

			log.Println("PAT", pat.ProgramMap(), pkt.PID())
			pmtPid, err = pat.SPTSpmtPID()
			if err != nil {
				log.Println("pmt pid", err)
				return errors.WithMessage(err, "Pmt pid")
			}
		}

		if packet.PayloadUnitStartIndicator(pkt) {
			if ok, _ := psi.IsPMT(pkt, pat); ok {
				PMT, err = psi.ReadPMT(bytes.NewReader(pkt[:]), pmtPid)
				if err != nil {
					log.Println("pmt", err)
					return errors.WithMessage(err, "Pmt")
				}

				for val, _ := range PMT.Pids() {
					wr := "#EXTINF:-1," + PMT.ElementaryStreams()[val].StreamTypeDescription() + "\n"
					if PMT.ElementaryStreams()[val].IsVideoContent() {
						video = PMT.ElementaryStreams()[val].ElementaryPid()
					}
					io.WriteString(M3u8, wr)
				}
			} else {
				data, err := packet.PESHeader(pkt)
				if err != nil {
					log.Println("pes", err)
					continue
				}
				PES[pkt.PID()] = pkt
				pusi, err := pes.NewPESHeader(data)

				if err != nil {
					log.Println("pusi", err)
					return err
				}
				PUSI[pkt.PID()] = pusi

				log.Println("PTS", pusi.PTS())
				if PTS[pkt.PID()] == 0 {
					PTS[pkt.PID()] = pusi.PTS()
				} else {
					if PTS[pkt.PID()] < pusi.PTS() {
						//todo ошибка pts любого фрейма в чанке в в любом элементарном потоке больше, чем первый фрейм соответствующего элементарного потока.
						io.WriteString(M3u8, "#EXT-X-DISCONTINUITY\n")
						log.Println("ошибка ПТС")
					}
				}
			}
		}

		if packet.ContainsAdaptationField(pkt) && packet.PayloadUnitStartIndicator(pkt) &&
			adaptationfield.IsRandomAccess(pkt) &&
			pkt.PID() == video {

			log.Println("random access", adaptationfield.IsRandomAccess(pkt))
			// todo показывает что содержится keyframe

			io.WriteString(M3u8, "#EXTINF:10, no desc\n")
			io.WriteString(M3u8, File.Name())
			io.WriteString(M3u8, "\n")
			File.Close()
			name++
			File, err = os.Create(strconv.Itoa(name) + ".ts")
			if err != nil {
				log.Fatal(err)
			}

			i = 0
		} else {
			i++
		}
		File.Write(pkt[:])
		log.Println("i", i)

	}

}

func main() {
	mpeg := os.Args[1]
	fmt.Println(mpeg)
	f, err := os.Open(mpeg)
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer f.Close()
	File, err := os.Create(mpeg + ".m3u8")
	if err != nil {
		log.Fatal(err)
	}
	log.Println(File)
	defer File.Close()
	go MpegTeg(f, File)
	for {

	}
}
