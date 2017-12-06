/*-
 * Copyright © 2017, Jörg Pernfuß <code.jpe@gmail.com>
 * All rights reserved.
 *
 * Use of this source code is governed by a 2-clause BSD license
 * that can be found in the LICENSE file.
 */

package mistral // import "github.com/mjolnir42/mistral/lib/mistral"

import (
	"strconv"

	"github.com/Shopify/sarama"
	"github.com/mjolnir42/erebos"
)

// process sends the received message to Kafka
func (m *Mistral) process(msg *erebos.Transport) {
	_, _, err := m.producer.SendMessage(&sarama.ProducerMessage{
		Topic: m.Config.Kafka.ProducerTopic,
		Key: sarama.StringEncoder(
			strconv.Itoa(int(msg.HostID)),
		),
		Value: sarama.ByteEncoder(msg.Value),
	})
	msg.Return <- err
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
