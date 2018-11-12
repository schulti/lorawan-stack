// Copyright © 2018 The Things Network Foundation, The Things Industries B.V.
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

package web

import (
	"bytes"
	"context"
	"net/http"
	"net/url"
	"path"
	"sync"

	"go.thethings.network/lorawan-stack/pkg/applicationserver/io"
	"go.thethings.network/lorawan-stack/pkg/errors"
	"go.thethings.network/lorawan-stack/pkg/log"
	"go.thethings.network/lorawan-stack/pkg/ttnpb"
)

// WebhookSink processes HTTP requests.
type WebhookSink interface {
	Process(*http.Request) error
}

// HTTPClientSink contains an HTTP client to make outgoing requests.
type HTTPClientSink struct {
	*http.Client
}

var errRequest = errors.DefineUnavailable("request", "request failed with status `{code}`")

// Process uses the HTTP client to perform the request.
func (s *HTTPClientSink) Process(req *http.Request) error {
	res, err := s.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 200 && res.StatusCode <= 299 {
		return nil
	}
	return errRequest.WithAttributes("code", res.StatusCode)
}

// Webhooks can be used to create a webhooks subscription.
type Webhooks struct {
	Registry WebhookRegistry
	Target   WebhookSink
}

// NewSubscription returns a new webhooks integration subscription.
func (w *Webhooks) NewSubscription(ctx context.Context) *io.Subscription {
	ctx = log.NewContextWithField(ctx, "namespace", "applicationserver/io/web")
	sub := io.NewSubscription(ctx, "webhook", nil)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-sub.Up():
				if err := w.handleUp(ctx, msg); err != nil {
					log.FromContext(ctx).WithError(err).Warn("Failed to handle message")
				}
			}
		}
	}()
	return sub
}

func (w *Webhooks) handleUp(ctx context.Context, msg *ttnpb.ApplicationUp) error {
	hooks, err := w.Registry.List(ctx, msg.ApplicationIdentifiers,
		[]string{
			"base_url",
			"headers",
			"formatter",
			"uplink_message",
			"join_accept",
			"downlink_ack",
			"downlink_nack",
			"downlink_sent",
			"downlink_failed",
			"downlink_queued",
			"location_solved",
		},
	)
	if err != nil {
		return err
	}
	wg := sync.WaitGroup{}
	for i := range hooks {
		hook := hooks[i]
		logger := log.FromContext(ctx).WithField("hook", hook.WebhookID)
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := w.newRequest(ctx, msg, hook)
			if err != nil {
				logger.WithError(err).Warn("Failed to create request")
				return
			}
			if req == nil {
				return
			}
			logger.WithField("url", req.URL).Debug("Processing message")
			if err := w.Target.Process(req); err != nil {
				logger.WithError(err).Warn("Failed to process message")
			}
		}()
	}
	wg.Wait()
	return nil
}

func (w *Webhooks) newRequest(ctx context.Context, msg *ttnpb.ApplicationUp, hook *ttnpb.ApplicationWebhook) (*http.Request, error) {
	var cfg *ttnpb.ApplicationWebhook_Message
	switch msg.Up.(type) {
	case *ttnpb.ApplicationUp_UplinkMessage:
		cfg = hook.UplinkMessage
	case *ttnpb.ApplicationUp_JoinAccept:
		cfg = hook.JoinAccept
	case *ttnpb.ApplicationUp_DownlinkAck:
		cfg = hook.DownlinkAck
	case *ttnpb.ApplicationUp_DownlinkNack:
		cfg = hook.DownlinkNack
	case *ttnpb.ApplicationUp_DownlinkSent:
		cfg = hook.DownlinkSent
	case *ttnpb.ApplicationUp_DownlinkFailed:
		cfg = hook.DownlinkFailed
	case *ttnpb.ApplicationUp_DownlinkQueued:
		cfg = hook.DownlinkQueued
	case *ttnpb.ApplicationUp_LocationSolved:
		cfg = hook.LocationSolved
	}
	if cfg == nil {
		return nil, nil
	}
	url, err := url.Parse(hook.BaseURL)
	if err != nil {
		return nil, err
	}
	url.Path = path.Join(url.Path, cfg.Path)
	formatter, ok := formatters[hook.Formatter]
	if !ok {
		return nil, errFormatterNotFound.WithAttributes("formatter", hook.Formatter)
	}
	buf, err := formatter.Encode(ctx, msg)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url.String(), bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", formatter.ContentType())
	for key, value := range hook.Headers {
		req.Header.Set(key, value)
	}
	return req, nil
}