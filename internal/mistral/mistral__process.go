/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package mistral // import "github.com/mjolnir42/mistral/internal/mistral"

import (
	"strconv"

	"github.com/Shopify/sarama"
	"github.com/mjolnir42/erebos"
	uuid "github.com/satori/go.uuid"
)

// process sends the received message to Kafka
func (m *Mistral) process(msg *erebos.Transport) {
	trackingID := uuid.NewV4().String()

	m.delay.Use()
	go func(hostID int, trackID string, data []byte) {
		m.dispatch <- &sarama.ProducerMessage{
			Topic: m.Config.Kafka.ProducerTopic,
			Key: sarama.StringEncoder(
				strconv.Itoa(hostID),
			),
			Value:    sarama.ByteEncoder(data),
			Metadata: trackID,
		}
		m.delay.Done()
	}(int(msg.HostID), trackingID, msg.Value)
	m.trackID[trackingID] = msg
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
