package engineio

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestServer(t *testing.T) {
	t1 := newFakeTransportCreater(true, "t1")
	t2 := newFakeTransportCreater(true, "t2")
	t3 := newFakeTransportCreater(false, "t3")
	registerTransport("t1", true, t1)
	registerTransport("t2", false, t2)
	registerTransport("t3", true, t3)

	Convey("Create server", t, func() {
		server, err := NewServer([]string{"t1", "t2", "t3"})
		So(err, ShouldBeNil)

		Convey("Test new id", func() {
			req, err := http.NewRequest("GET", "/", nil)
			So(err, ShouldBeNil)
			id1 := server.newId(req)
			id2 := server.newId(req)
			So(id1, ShouldNotEqual, id2)
		})

		Convey("Test on close", func() {
			req, err := http.NewRequest("GET", "/", nil)
			t, err := t1(req)
			So(err, ShouldBeNil)
			id := "abc"
			conn, err := newSocket(id, server, t, req)
			So(err, ShouldBeNil)
			server.sessions.Set(id, conn)
			server.onClose(conn)
			So(server.sessions.Get(id), ShouldBeNil)
		})

		Convey("Test serve http", func() {

			Convey("Normal request", func() {
				check := make(chan bool)
				id := ""
				go func() {
					conn, _ := server.Accept()
					id = conn.Id()
					conn.Close()
					check <- true
				}()

				p := make(url.Values)
				p.Set("EIO", fmt.Sprintf("%d", Protocol))
				p.Set("transport", "t1")
				p.Set("t", fmt.Sprintf("%d-0", time.Now().Unix()))

				r, err := http.NewRequest("GET", "/?"+p.Encode(), bytes.NewBuffer(nil))
				So(err, ShouldBeNil)
				w := httptest.NewRecorder()

				server.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusOK)

				<-check

				p.Set("sid", id)
				r, err = http.NewRequest("GET", "/?"+p.Encode(), bytes.NewBuffer(nil))
				So(err, ShouldBeNil)
				w = httptest.NewRecorder()

				server.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusOK)
			})

			Convey("Not allowed", func() {
				s, err := NewServer(nil)
				So(err, ShouldBeNil)
				s.SetAllowRequest(func(*http.Request) error {
					return errors.New("not allowed")
				})

				p := make(url.Values)
				p.Set("EIO", fmt.Sprintf("%d", Protocol))
				p.Set("transport", "t1")
				p.Set("t", fmt.Sprintf("%d-0", time.Now().Unix()))

				r, err := http.NewRequest("GET", "/?"+p.Encode(), bytes.NewBuffer(nil))
				So(err, ShouldBeNil)
				w := httptest.NewRecorder()

				s.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusBadRequest)
				So(w.Body.String(), ShouldEqual, "not allowed\n")
			})

			Convey("Wrong transport", func() {
				p := make(url.Values)
				p.Set("EIO", fmt.Sprintf("%d", Protocol))
				p.Set("transport", "notexist")
				p.Set("t", fmt.Sprintf("%d-0", time.Now().Unix()))

				r, err := http.NewRequest("GET", "/?"+p.Encode(), bytes.NewBuffer(nil))
				So(err, ShouldBeNil)
				w := httptest.NewRecorder()

				server.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusBadRequest)
				So(w.Body.String(), ShouldEqual, "invalid transport\n")
			})

			Convey("Transport error", func() {
				p := make(url.Values)
				p.Set("EIO", fmt.Sprintf("%d", Protocol))
				p.Set("transport", "t3")
				p.Set("t", fmt.Sprintf("%d-0", time.Now().Unix()))

				r, err := http.NewRequest("GET", "/?"+p.Encode(), bytes.NewBuffer(nil))
				So(err, ShouldBeNil)
				w := httptest.NewRecorder()

				server.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusBadRequest)
				So(w.Body.String(), ShouldEqual, "transport t3 error\n")
			})

			Convey("Wrong session id", func() {
				check := make(chan bool)
				var conn Conn
				go func() {
					conn, _ = server.Accept()
					conn.Close()
					check <- true
				}()

				p := make(url.Values)
				p.Set("EIO", fmt.Sprintf("%d", Protocol))
				p.Set("transport", "t1")
				p.Set("t", fmt.Sprintf("%d-0", time.Now().Unix()))

				r, err := http.NewRequest("GET", "/?"+p.Encode(), bytes.NewBuffer(nil))
				So(err, ShouldBeNil)
				w := httptest.NewRecorder()

				server.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusOK)

				<-check
				defer conn.Close()

				p.Set("sid", conn.Id()+"abc")
				r, err = http.NewRequest("GET", "/?"+p.Encode(), bytes.NewBuffer(nil))
				So(err, ShouldBeNil)
				w = httptest.NewRecorder()

				server.ServeHTTP(w, r)
				So(w.Code, ShouldEqual, http.StatusBadRequest)
				So(w.Body.String(), ShouldEqual, "invalid sid\n")
			})

		})

	})
}
