/*-
 * Copyright (c) 2013 Julien Schmidt. All rights reserved.
 *
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions are met:
 *     * Redistributions of source code must retain the above copyright
 *       notice, this list of conditions and the following disclaimer.
 *     * Redistributions in binary form must reproduce the above copyright
 *       notice, this list of conditions and the following disclaimer in the
 *       documentation and/or other materials provided with the distribution.
 *     * The names of the contributors may not be used to endorse or promote
 *       products derived from this software without specific prior written
 *       permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
 * WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
 * DISCLAIMED. IN NO EVENT SHALL JULIEN SCHMIDT BE LIABLE FOR ANY
 * DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
 * (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
 * LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
 * ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
 * SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */

package main // import "github.com/mjolnir42/mistral/cmd/mistral"

// This function is part of the README examples of julienschmidt/httprouter

import (
	"bytes"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
)

// BasicAuth performs HTTP Basic authentication uses credentials stored
// as private package variables
func BasicAuth(h httprouter.Handle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		const basicAuthPrefix string = `Basic `

		// Get the Basic Authentication credentials
		auth := r.Header.Get(`Authorization`)
		if strings.HasPrefix(auth, basicAuthPrefix) {
			// Check credentials
			payload, err := base64.StdEncoding.DecodeString(auth[len(basicAuthPrefix):])
			if err == nil {
				pair := bytes.SplitN(payload, []byte(`:`), 2)
				if len(pair) == 2 &&
					subtle.ConstantTimeCompare(pair[0], []byte(basicAuthUsername)) == 1 &&
					subtle.ConstantTimeCompare(pair[1], []byte(basicAuthPassword)) == 1 {

					// Delegate request to the given handle
					h(w, r, ps)
					return
				}
			}
		}

		// Request Basic Authentication otherwise
		w.Header().Set(`WWW-Authenticate`, `Basic realm=Restricted`)
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
	}
}

// vim: ts=4 sw=4 sts=4 noet fenc=utf-8 ffs=unix
