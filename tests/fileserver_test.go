package hello_test

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const fileserverAddress = "http://localhost:28081"

var fileClient = http.Client{
	Timeout: 10 * time.Second,
}

type file struct {
	content []byte
	name    string
}

var files = []file{
	{
		content: []byte("go, go Gophers!"),
		name:    "file1.txt",
	},
	{
		content: []byte("Hi!\nBye!\n"),
		name:    "file2.txt",
	},
}

func createFiles() error {
	for _, f := range files {
		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		part, err := writer.CreateFormFile("file", f.name)
		if err != nil {
			return err
		}
		_, err = part.Write(f.content)
		if err != nil {
			return err
		}
		err = writer.Close()
		if err != nil {
			return err
		}
		createUrl, err := url.JoinPath(fileserverAddress, "files")
		if err != nil {
			return err
		}
		request, err := http.NewRequest(http.MethodPost, createUrl, body)
		if err != nil {
			return err
		}
		request.Header.Add("Content-Type", writer.FormDataContentType())
		response, err := fileClient.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusCreated {
			return fmt.Errorf(
				"wrong status code: expected %d, received %d",
				http.StatusCreated, response.StatusCode,
			)
		}
		data, err := io.ReadAll(response.Body)
		if err != nil {
			return err
		}
		id := strings.TrimSpace(string(data))
		if id != f.name {
			return fmt.Errorf(
				"wrong resource id returned: expected %s, received %s",
				f.name, id,
			)
		}
	}
	return nil
}

func deleteFiles() error {
	for _, f := range files {
		deleteUrl, err := url.JoinPath(fileserverAddress, "files", f.name)
		if err != nil {
			return err
		}
		request, err := http.NewRequest(http.MethodDelete, deleteUrl, nil)
		if err != nil {
			return err
		}
		response, err := fileClient.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			return fmt.Errorf(
				"wrong status code: expected %d, received %d",
				http.StatusOK, response.StatusCode,
			)
		}
	}
	return nil
}

func TestFsCreateListDelete(t *testing.T) {
	defer deleteFiles()
	err := createFiles()
	require.NoError(t, err)

	listUrl, err := url.JoinPath(fileserverAddress, "files")
	require.NoError(t, err)
	response, err := fileClient.Get(listUrl)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, files[0].name+"\n"+files[1].name+"\n", string(data))

	err = deleteFiles()
	require.NoError(t, err)
}

func TestFsRead(t *testing.T) {
	defer deleteFiles()
	err := createFiles()
	require.NoError(t, err)

	readUrl, err := url.JoinPath(fileserverAddress, "files", files[0].name)
	require.NoError(t, err)
	response, err := fileClient.Get(readUrl)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, files[0].content, data)

	err = deleteFiles()
	require.NoError(t, err)
}

func TestFsReadNotExists(t *testing.T) {
	defer deleteFiles()
	err := createFiles()
	require.NoError(t, err)

	readUrl, err := url.JoinPath(fileserverAddress, "files", "Unknown")
	require.NoError(t, err)
	response, err := fileClient.Get(readUrl)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusNotFound, response.StatusCode)
	err = deleteFiles()
	require.NoError(t, err)
}

func TestFsCreateAlreadyExists(t *testing.T) {
	defer deleteFiles()
	err := createFiles()
	require.NoError(t, err)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", files[0].name)
	require.NoError(t, err)
	_, err = part.Write(files[0].content)
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	createUrl, err := url.JoinPath(fileserverAddress, "files")
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPost, createUrl, body)
	require.NoError(t, err)
	request.Header.Add("Content-Type", writer.FormDataContentType())
	response, err := fileClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusConflict, response.StatusCode)

	err = deleteFiles()
	require.NoError(t, err)
}

func TestFsUpdate(t *testing.T) {
	defer deleteFiles()
	err := createFiles()
	require.NoError(t, err)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", files[0].name)
	require.NoError(t, err)
	_, err = part.Write(files[1].content)
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	updateUrl, err := url.JoinPath(fileserverAddress, "files", files[0].name)
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPut, updateUrl, body)
	require.NoError(t, err)
	request.Header.Add("Content-Type", writer.FormDataContentType())
	response, err := fileClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)

	readUrl, err := url.JoinPath(fileserverAddress, "files", files[0].name)
	require.NoError(t, err)
	response, err = fileClient.Get(readUrl)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusOK, response.StatusCode)
	data, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, files[1].content, data)

	err = deleteFiles()
	require.NoError(t, err)
}

func TestFsUpdateNotExists(t *testing.T) {
	defer deleteFiles()
	err := createFiles()
	require.NoError(t, err)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "Unknown")
	require.NoError(t, err)
	_, err = part.Write(files[1].content)
	require.NoError(t, err)
	err = writer.Close()
	require.NoError(t, err)

	updateUrl, err := url.JoinPath(fileserverAddress, "files", "Unknown")
	require.NoError(t, err)
	request, err := http.NewRequest(http.MethodPut, updateUrl, body)
	require.NoError(t, err)
	request.Header.Add("Content-Type", writer.FormDataContentType())
	response, err := fileClient.Do(request)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusNotFound, response.StatusCode)

	err = deleteFiles()
	require.NoError(t, err)
}
