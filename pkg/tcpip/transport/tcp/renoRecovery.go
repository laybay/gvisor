// Copyright 2020 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tcp

// renoRecovery stores the variables related to TCP Reno loss recovery
// algorithm.
//
// +stateify savable
type renoRecovery struct {
	s *sender
}

func newRenoRecovery(s *sender) *renoRecovery {
	return &renoRecovery{s: s}
}

func (rr *renoRecovery) Update() {
	rr.s.state = FastRecovery
	rr.s.ep.stack.Stats().TCP.FastRecovery.Increment()
}

func (rr *renoRecovery) DoRecovery(seg *segment, rtx bool) {
	ack := seg.ackNumber

	// We are in fast recovery mode. Ignore the ack if it's out of
	// range.
	if !ack.InRange(rr.s.sndUna, rr.s.sndNxt+1) {
		return
	}

	// Don't count this as a duplicate if it is carrying data or
	// updating the window.
	if seg.logicalLen() != 0 || rr.s.sndWnd != seg.window {
		return
	}

	// Inflate the congestion window if we're getting duplicate acks
	// for the packet we retransmitted.
	if !rtx && ack == rr.s.fr.first {
		// We received a dup, inflate the congestion window by 1 packet
		// if we're not at the max yet. Only inflate the window if
		// regular FastRecovery is in use, RFC6675 does not require
		// inflating cwnd on duplicate ACKs.
		if rr.s.sndCwnd < rr.s.fr.maxCwnd {
			rr.s.sndCwnd++
		}
		return
	}

	// A partial ack was received. Retransmit this packet and
	// remember it so that we don't retransmit it again. We don't
	// inflate the window because we're putting the same packet back
	// onto the wire.
	//
	// N.B. The retransmit timer will be reset by the caller.
	rr.s.fr.first = ack
	rr.s.dupAckCount = 0
	rr.s.resendSegment()
}