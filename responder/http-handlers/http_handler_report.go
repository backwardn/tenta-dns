/**
 * Tenta DNS Server
 *
 *    Copyright 2017 Tenta, LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * For any questions, please contact developer@tenta.io
 *
 * http_handler_report.go: NSNitch Report API
 */

package http_handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/tenta-browser/tenta-dns/common"
	"github.com/tenta-browser/tenta-dns/runtime"

	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
)

func HandleHTTPReport(cfg runtime.NSnitchConfig, rt *runtime.Runtime, d *runtime.ServerDomain, lgr *logrus.Entry) httpHandler {
	return wrapExtendedHttpHandler(rt, lgr, "report", func(w http.ResponseWriter, r *http.Request, lg *logrus.Entry) {
		key := []byte(fmt.Sprintf("query/%s", r.Host))
		if r.Host != strings.ToLower(r.Host) {
			lg.Debugf("Hostname contains uppercase %s", r.Host)
			key = []byte(fmt.Sprintf("query/%s", strings.ToLower(r.Host)))
		}

		if !dns.IsSubDomain(d.HostName, r.Host) {
			lg.Warnf("Handling request for invalid domain. Serving: %s, Requested: %s", d.HostName, r.Host)
			HandleHTTPDefault(cfg, rt, lg)(w, r)
			return
		}

		data := &common.DefaultJSONObject{
			Status:  "OK",
			Type:    "TENTA_NSNITCH",
			Data:    nil,
			Message: "",
			Code:    200,
		}

		_, err := rt.DBGet(common.AddSuffix(key, runtime.KEY_NAME))
		if err != nil {
			// Fix for race where this request completes before the DNS goroutine has finished writing the DB record
			time.Sleep(500 * time.Millisecond)
			_, err := rt.DBGet(common.AddSuffix(key, runtime.KEY_NAME))
			if err != nil {
				lg.Warnf("DB Error: %s", err.Error())
				data.Status = "ERROR"
				if err == leveldb.ErrNotFound {
					data.Message = "Not Found"
					data.Code = http.StatusNotFound
				} else {
					data.Message = "Internal Error"
					data.Code = http.StatusInternalServerError
				}
				extraHeaders(cfg, w, r)
				w.WriteHeader(data.Code)
				mustMarshall(w, data, lg)
				return
			}
		}
		recarr, _ := rt.DBGet(common.AddSuffix(key, runtime.KEY_DATA))
		rec := &common.DNSTelemetry{}
		json.Unmarshal(recarr, rec)

		data.Data = rec

		extraHeaders(cfg, w, r)
		mustMarshall(w, data, lg)
	})
}
