package storage_test

import (
	. "github.com/gavrilaf/chuck/storage"
	. "github.com/gavrilaf/chuck/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"bufio"
	"fmt"
	"github.com/mitchellh/cli"
	"github.com/spf13/afero"
	"net/http"
)

var _ = Describe("Recorder", func() {
	var (
		log     Logger
		subject Recorder
		root    *afero.Afero

		createRequest  func(string) *http.Request
		createResponse func() *http.Response
	)

	BeforeEach(func() {
		createRequest = func(method string) *http.Request {
			header := make(http.Header)
			header.Set("Content-Type", "application/json")
			req, _ := MakeRequest2(method, "https://secure.api.com?query=123", header, "{}")
			return req
		}

		createResponse = func() *http.Response {
			body := `{"colors": []}`
			header := make(http.Header)
			header.Set("Content-Type", "application/json")
			header.Set("Content-Length", "6573")

			return MakeResponse2(200, header, body)
		}

		log = NewLogger(cli.NewMockUi())

		fs := afero.NewMemMapFs()
		root = &afero.Afero{Fs: fs}
	})

	Describe("Create Recorder", func() {
		var (
			err         error
			dirExists   bool
			indexExists bool
		)

		Context("when createNewFolder is true", func() {
			BeforeEach(func() {
				subject, err = NewRecorderWithFs(root.Fs, "log-1", true, false, log)

				path := "log-1/" + subject.Name()
				dirExists, _ = root.DirExists(path)
				indexExists, _ = root.Exists(path + "/index.txt")
			})

			It("should not error occurred", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return Recorder object", func() {
				Expect(subject).ToNot(BeNil())
			})

			It("should create a recorder root logger folder", func() {
				Expect(dirExists).To(BeTrue())
			})

			It("should create an index file", func() {
				Expect(indexExists).To(BeTrue())
			})
		})

		Context("when createNewFolder is false", func() {
			BeforeEach(func() {
				subject, err = NewRecorderWithFs(root.Fs, "log-2", false, false, log)
				indexExists, _ = root.Exists("log-2/index.txt")
			})

			It("should not error occurred", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should create an index file", func() {
				Expect(indexExists).To(BeTrue())
			})
		})
	})

	Describe("Recording", func() {
		var (
			basePath string
			dumpPath string
			session  int64
			req      *http.Request
			resp     *http.Response

			err        error
			reqResult  *PendingRequest
			respResult *PendingRequest
		)

		BeforeEach(func() {
			subject, _ = NewRecorderWithFs(root.Fs, "log-3", true, false, log)
			basePath = "log-3/" + subject.Name()
			session = 10
			req = createRequest("POST")
			resp = createResponse()
		})

		It("should contains no pending requests", func() {
			Expect(subject.PendingCount()).To(Equal(0))
		})

		Describe("Record request", func() {
			BeforeEach(func() {
				reqResult, err = subject.RecordRequest(req, session)
				dumpPath = fmt.Sprintf("%s/r_%d/", basePath, reqResult.Id)
			})

			It("should not error occurred", func() {
				Expect(err).ToNot(HaveOccurred())
			})

			It("should return pending request", func() {
				Expect(reqResult).ToNot(BeNil())
			})

			It("should create request dump folder", func() {
				dirExists, _ := root.DirExists(dumpPath)
				Expect(dirExists).To(BeTrue())
			})

			It("should create request headers dump", func() {
				dumpExists, _ := root.Exists(dumpPath + "req_header.json")
				Expect(dumpExists).To(BeTrue())
			})

			It("should create request body dump", func() {
				dumpExists, _ := root.Exists(dumpPath + "req_body.json")
				Expect(dumpExists).To(BeTrue())
			})

			Describe("Record response", func() {
				BeforeEach(func() {
					respResult, err = subject.RecordResponse(resp, session)
				})

				It("should not error occurred", func() {
					Expect(err).ToNot(HaveOccurred())
				})

				It("should index.txt contains log record", func() {
					fi, _ := root.Open(basePath + "/" + "index.txt")
					defer fi.Close()

					scanner := bufio.NewScanner(fi)
					scanner.Scan()
					line := scanner.Text()

					expected := fmt.Sprintf("N,\t200,\tr_%d,\tPOST,\thttps://secure.api.com?query=123", respResult.Id)
					Expect(expected).To(Equal(line))
				})

				It("should create response headers dump", func() {
					dumpExists, _ := root.Exists(dumpPath + "resp_header.json")
					Expect(dumpExists).To(BeTrue())
				})

				It("should create response body dump", func() {
					dumpExists, _ := root.Exists(dumpPath + "resp_body.json")
					Expect(dumpExists).To(BeTrue())
				})

				Describe("Create new_only recorder based on the same dir", func() {
					var (
						subjectNew Recorder
					)

					BeforeEach(func() {
						subjectNew, err = NewRecorderWithFs(root.Fs, basePath, false, true, log)
					})

					It("should not error occurred", func() {
						Expect(err).ToNot(HaveOccurred())
					})

					It("should create recorder", func() {
						Expect(subjectNew).ToNot(BeNil())
					})

					Context("when record the request with the same method and url", func() {
						BeforeEach(func() {
							reqResult, err = subjectNew.RecordRequest(req, 101)
						})

						It("should not error occurred", func() {
							Expect(err).ToNot(HaveOccurred())
						})

						It("should not return pending request", func() {
							Expect(reqResult).To(BeNil())
						})
					})

					Context("when record the request with the new method", func() {
						BeforeEach(func() {
							req = createRequest("PUT")
							reqResult, err = subjectNew.RecordRequest(req, 102)
						})

						It("should not error occurred", func() {
							Expect(err).ToNot(HaveOccurred())
						})

						It("should return the pending request", func() {
							Expect(reqResult).ToNot(BeNil())
						})

						Describe("record the response", func() {
							BeforeEach(func() {
								respResult, err = subjectNew.RecordResponse(resp, 102)
							})

							It("should not error occurred", func() {
								Expect(err).ToNot(HaveOccurred())
							})

							It("should return the same result", func() {
								Expect(reqResult).To(Equal(respResult))
							})
						})
					})
				})
			})

			Describe("Record focused", func() {
				BeforeEach(func() {
					subject.SetFocusedMode(true)

					reqResult, _ = subject.RecordRequest(req, session)
					subject.RecordResponse(resp, session)
				})

				It("should record request as focused", func() {
					fi, _ := root.Open(basePath + "/" + "index.txt")
					defer fi.Close()
					scanner := bufio.NewScanner(fi)
					scanner.Scan()
					line := scanner.Text()

					expected := fmt.Sprintf("F,\t200,\tr_%d,\tPOST,\thttps://secure.api.com?query=123", reqResult.Id)
					Expect(expected).To(Equal(line))
				})
			})
		})

		Describe("Recording in the new_only mode", func() {
			var (
				basePath string
			)

			BeforeEach(func() {
				subject, err = NewRecorderWithFs(root.Fs, "log-4", true, true, log)
				basePath = "log-4/" + subject.Name()

				req := createRequest("POST")
				resp := createResponse()

				subject.RecordRequest(req, 10)
				subject.RecordResponse(resp, 10)

				subject.RecordRequest(req, 11)
				subject.RecordResponse(resp, 11)

				req = createRequest("GET")

				subject.RecordRequest(req, 12)
				subject.RecordResponse(resp, 12)
			})

			It("should not record second response", func() {
				fi, _ := root.Open(basePath + "/" + "index.txt")
				defer fi.Close()

				scanner := bufio.NewScanner(fi)

				Expect(scanner.Scan()).To(BeTrue())
				Expect(scanner.Text()).To(Equal(fmt.Sprintf("N,\t200,\tr_1,\tPOST,\thttps://secure.api.com?query=123")))

				Expect(scanner.Scan()).To(BeTrue())
				Expect(scanner.Text()).To(Equal(fmt.Sprintf("N,\t200,\tr_2,\tGET,\thttps://secure.api.com?query=123")))

				Expect(scanner.Scan()).To(BeFalse(), "should contain only two records")
			})
		})
	})
})
