package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestRemotePlaylistCoverIsLazyAuthenticatedAndBoundToOriginalServer(t *testing.T) {
	var calls atomic.Int64
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/videos/42/playlist-cover" || r.URL.Query().Get("location_id") != "7" || r.Header.Get("Cookie") != "session=secret" {
			t.Errorf("wrong request: %s %s", r.URL, r.Header.Get("Cookie"))
			w.WriteHeader(403)
			return
		}
		w.Header().Set("X-JavBoss-Cover-Aspect", "0.7")
		w.Header().Set("X-JavBoss-Cover-Position", "0.8")
		_, _ = w.Write([]byte("cached image"))
	}))
	defer remote.Close()
	c := newTestClient(t, remote.URL, nil)
	resolve := c.playlistCoverResolver(c.remoteState(), 42, 7, "session=secret")
	if calls.Load() != 0 {
		t.Fatal("fetched covers before they were visible")
	}
	source, err := resolve(context.Background())
	if err != nil || string(source.Data) != "cached image" || source.Path != "" || source.CropPosition != 0.8 || source.CropAspect != 0.7 {
		t.Fatalf("source=%+v err=%v", source, err)
	}
	c.remoteMu.Lock()
	c.remote = &remoteState{}
	c.remoteMu.Unlock()
	if _, err := resolve(context.Background()); err == nil || calls.Load() != 1 {
		t.Fatal("reused credentials after remote changed")
	}
}

func TestRemotePlaylistCoverNeverFollowsRedirects(t *testing.T) {
	var leaked atomic.Int64
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked.Add(1) }))
	defer other.Close()
	remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, 302) }))
	defer remote.Close()
	c := newTestClient(t, remote.URL, nil)
	if _, err := c.playlistCoverResolver(c.remoteState(), 42, 0, "session=secret")(context.Background()); err == nil {
		t.Fatal("redirect accepted")
	}
	if leaked.Load() != 0 {
		t.Fatal("credentials sent to redirect target")
	}
}
