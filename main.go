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
	//var patPkt *packet.Packet
	var pat psi.PAT
	pmtPkt := make(map[int]*packet.Packet)
	var PAT *packet.Packet
	pmtPids := make(map[int]psi.PmtElementaryStream)
	PUSI := make(map[int]pes.PESHeader)
	pmtPid := 0
	var ts []packet.Packet
	ts = make([]packet.Packet, 0)
	var i int
	var name int

	io.WriteString(M3u8, "#EXTM3U\n")
	io.WriteString(M3u8, "#EXT-X-TARGETDURATION:10\n")
	io.WriteString(M3u8, "#EXT-X-MEDIA-SEQUENCE:1\n")
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
			pkt.SetTransportErrorIndicator(true)
		}
		Cc[pkt.PID()] = pkt.ContinuityCounter()
		//fmt.Printf("mpeg %+v %v %v %v %v\n",pkt.CheckErrors(), pkt.HasPayload(), pkt.IsPAT(), pkt.PayloadUnitStartIndicator(),pkt.ContinuityCounter())
		if packet.IsPat(pkt) {
			PAT = pkt
			pat, err = psi.ReadPAT(bytes.NewReader(pkt[:]))
			//patPkt = pkt

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
		if ok, _ := psi.IsPMT(pkt, pat); ok {
			pmtPkt[pmtPid] = pkt
			PMT, err := psi.ReadPMT(bytes.NewReader(pkt[:]), pmtPid)
			if err != nil {
				log.Println("pmt", err)
				return errors.WithMessage(err, "Pmt")
			}
			for val, pid := range PMT.Pids() {
				pmtPids[pid] = PMT.ElementaryStreams()[val]
				wr := "#EXTINF:-1," + PMT.ElementaryStreams()[val].StreamTypeDescription() + "\n"
				io.WriteString(M3u8, wr)
			}
		}

		ts = append(ts, *pkt)

		if packet.PayloadUnitStartIndicator(pkt) {
			data, err := packet.PESHeader(pkt)
			if err != nil {
				log.Println("pes", err)
				continue
			}
			pusi, err := pes.NewPESHeader(data)

			if err != nil {
				log.Println("pusi", err)
				return err
			}
			//payload = new(packet.Packet)
			PUSI[pkt.PID()] = pusi

			log.Println("PTS", pusi.PTS())
			if PTS[pkt.PID()] == 0 {
				PTS[pkt.PID()] = pusi.PTS()
			} else {
				if PTS[pkt.PID()] < pusi.PTS() {
					//todo ошибка pts любого фрейма в чанке в в любом элементарном потоке больше, чем первый фрейм соответствующего элементарного потока.
					pkt.SetTransportErrorIndicator(true)
					log.Println("ошибка ПТС")
				}
			}
		}
		if packet.ContainsAdaptationField(pkt) && packet.PayloadUnitStartIndicator(pkt) && adaptationfield.IsRandomAccess(pkt) {
			log.Println("random access", adaptationfield.IsRandomAccess(pkt))
			// todo показывает что содержится keyframe
			var j int

			if len(ts) < 5 {
				continue
			}
			File, err := os.Create(strconv.Itoa(name) + ".ts")
			if err != nil {
				log.Fatal(err)
			}
			name++
			log.Println(File)
			flag := false
			File.Write(PAT[:])
			File.Write(pmtPkt[pmtPid][:])

			for j < len(ts) {
				File.Write((&ts[j])[:])

				if ts[j].TransportErrorIndicator() {
					flag = true
				}
				//ts[i].SetPID(pkt.PID())
				log.Println("j", j)
				j++
			}
			File.Close()
			if flag {
				io.WriteString(M3u8, "#EXT-X-DISCONTINUITY\n")
			}
			io.WriteString(M3u8, "#EXTINF:10, no desc\n")
			io.WriteString(M3u8, File.Name())
			io.WriteString(M3u8, "\n")
			ts = make([]packet.Packet, 0)
			i = 0
		} else {
			i++
		}
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
