// Package pages provides some helpers for serving web pages.
//
// Example usage:
//   var page = Add("/uri", handler, "tmpl/base.tmpl", "tmpl/page.tmpl")
//
//   func handler(w http.ResponseWriter, r *http.Request) pages.Result {
//     return pages.OK("some data to page.tmpl")
//   }
//
//   pages.SetLogger(func(r *http.Request) pages.Logger {
//     return appengine.NewContext(r)
//   })
//   http.Handle(page.URI, page)
package pages

import (
	"html/template"
	"net/http"

	"log"
)

var (
	logger              LoggerFunc
	BaseTemplate        = "base" // name of top-level template to invoke for each page
	StatusBadRequest    = Result{responseCode: http.StatusBadRequest}
	StatusNotFound      = Result{responseCode: http.StatusNotFound}
	StatusInternalError = Result{responseCode: http.StatusInternalServerError}
)

// SetLogger registers a logger function.
func SetLogger(l LoggerFunc) {
	logger = l
}

// A Page to be rendered.
type Page struct {
	URI    string             // URI path
	Render Renderer           // func to render the page
	tmpl   *template.Template // backing template
}

// Renderer is a function to render a page result.
type Renderer func(w http.ResponseWriter, r *http.Request) Result

// Add creates a new page.
//
// Add panics if the page templates cannot be parsed.
func Add(uri string, render Renderer, tmpls ...string) Page {
	t := template.Must(template.ParseFiles(tmpls...))
	return Page{
		URI:    uri,
		tmpl:   t,
		Render: render,
	}
}

// Result is the result of rendering a page.
type Result struct {
	data         interface{} // Data to render the page.
	responseCode int         // HTTP response code.
	err          error       // Error, or nil.
}

// StatusOK returns http.StatusOK with given data passed to the template.
func StatusOK(data interface{}) Result {
	return Result{
		responseCode: http.StatusOK,
		data:         data,
	}
}

// BadRequestWith returns a Result indicating a bad request.
func BadRequestWith(err error) Result {
	return Result{
		responseCode: http.StatusBadRequest,
		err:          err,
	}
}

// ServeHTTP serves HTTP for the page.
//
// ServeHTTP panics if no logger has been registered with SetLogger.
func (p Page) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if logger == nil {
		log.Fatalf("no logger specified; call SetLogger\n")
	}
	l := logger(r)
	l.Infof("Page %+v will ServeHTTP for URL: %v", p, r.URL)

	// Render the page, retrieving any data for the template.
	pr := p.Render(w, r)
	if pr.err != nil || pr.responseCode != http.StatusOK {
		if pr.err != nil {
			l.Errorf("Error while rendering %v: %v\n", r.URL, pr.err)
		}
		if pr.responseCode == http.StatusNotFound {
			http.NotFound(w, r)
		} else if pr.responseCode == http.StatusBadRequest {
			http.Error(w, "Bad request", http.StatusBadRequest)
		} else {
			http.Error(w, "Internal server error.", pr.responseCode)
		}
		return
	}

	err := p.tmpl.ExecuteTemplate(w, BaseTemplate, pr.data)
	if err != nil {
		// TODO: If this happens, partial template data is still written
		// to w by ExecuteTemplate, which isn't ideal; we'd like the 500
		// to be the only thing returned to viewing user.

		// Error rendering the template is a programming bug.
		l.Errorf("Failed to render template: %v", err)
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
	}
}

// Logger specifies logging functions.
type Logger interface {
	// Debugf formats its arguments according to the format, analogous to fmt.Printf,
	// and records the text as a log message at Debug level.
	Debugf(format string, args ...interface{})

	// Infof is like Debugf, but at Info level.
	Infof(format string, args ...interface{})

	// Warningf is like Debugf, but at Warning level.
	Warningf(format string, args ...interface{})

	// Errorf is like Debugf, but at Error level.
	Errorf(format string, args ...interface{})

	// Criticalf is like Debugf, but at Critical level.
	Criticalf(format string, args ...interface{})
}

// LoggerFunc returns a logger from a http request.
type LoggerFunc func(*http.Request) Logger
