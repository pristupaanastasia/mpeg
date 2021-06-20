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
)

var Cc map[int]int
var PTS map[int]uint64

func MpegTeg(r io.Reader) error {
	Cc = make(map[int]int, 0)
	PTS = make(map[int]uint64, 0)
	pktData := make([]byte, packet.PacketSize)
	//var patPkt *packet.Packet
	var pat psi.PAT
	var pmtPkt *packet.Packet
	pmtPids := make(map[int]psi.PmtElementaryStream)
	payload := make(map[int]packet.Packet)
	pmtPid := 0

	for {
		_, err := io.ReadFull(r, pktData)
		if err != nil {
			if err == io.EOF {
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

		if (pkt.ContinuityCounter()+1 != Cc[pkt.PID()] && pkt.ContinuityCounter() != 0) || (pkt.ContinuityCounter() == 0 && Cc[pkt.PID()] != 15) {
			//todo здесь будет вставка ошибки в hls - сс пропущен
		}
		Cc[pkt.PID()] = pkt.ContinuityCounter()
		//fmt.Printf("mpeg %+v %v %v %v %v\n",pkt.CheckErrors(), pkt.HasPayload(), pkt.IsPAT(), pkt.PayloadUnitStartIndicator(),pkt.ContinuityCounter())
		if packet.IsPat(pkt) {
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
			pmtPkt = pkt
			PMT, err := psi.ReadPMT(bytes.NewReader(pmtPkt[:]), pmtPid)
			if err != nil {
				log.Println("pmt", err)
				return errors.WithMessage(err, "Pmt")
			}
			for i, pid := range PMT.Pids() {
				pmtPids[pid] = PMT.ElementaryStreams()[i]
			}

		}

		if packet.PayloadUnitStartIndicator(pkt) {
			data, err := packet.PESHeader(pkt)
			if err != nil {
				log.Println(err)
				return err
			}
			pusi, err := pes.NewPESHeader(data)
			if err != nil {
				log.Println(err)
				return err
			}
			//payload = new(packet.Packet)
			payload[pkt.PID()] = *pkt

			log.Println("PTS", pusi.PTS())
			if PTS[pkt.PID()] == 0 {
				PTS[pkt.PID()] = pusi.PTS()
			} else {
				if PTS[pkt.PID()] < pusi.PTS() {
					//todo ошибка pts любого фрейма в чанке в в любом элементарном потоке больше, чем первый фрейм соответствующего элементарного потока.
				}
			}
		}
		if packet.ContainsAdaptationField(pkt){
			log.Println(adaptationfield.IsRandomAccess(pkt))
			// todo показывает что содержится keyframe
			adaptationfield.
		}
		ts := packet.New()
		ts.SetContinuityCounter(Cc[pkt.PID()])
		if pkt.HasPayload() {
			val, err := pkt.Payload()
			if err != nil {
				log.Println(err)
				return err
			}
			ts.SetPayload(val)
		}
		if pkt.HasAdaptationField() {
			val, err := ts.AdaptationField()
			if err != nil {
				log.Println(err)
				return err
			}
			log.Println("key frame")
			log.Println(val.RandomAccess())
			ts.SetAdaptationField(val)

		}
		ts.SetPID(pkt.PID())
		log.Println("ts", ts.PID(), ts.IsPAT(), ts.CheckErrors(), ts.HasPayload())
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
	go MpegTeg(f)
	for {

	}
}
