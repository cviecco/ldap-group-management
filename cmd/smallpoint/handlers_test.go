package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNonAdminWebPathsHandlerSuccess(t *testing.T) {

	state, err := setupTestState()
	if err != nil {
		log.Println(err)
	}
	validTestGroupInfoPath := groupinfoPath + "?groupname=group1"
	testWebEndpoints := map[string]http.HandlerFunc{
		indexPath:                  state.mygroupsHandler,
		allLDAPgroupsPath:          state.allGroupsHandler,
		myManagedGroupsWebPagePath: state.myManagedGroupsHandler,
		pendingactionsPath:         state.pendingActions,
		pendingrequestsPath:        state.pendingRequests,
		addmembersPath:             state.addmemberstoGroupWebpageHandler,
		deletemembersPath:          state.deletemembersfromGroupWebpageHandler,
		validTestGroupInfoPath:     state.groupInfoWebpage,
		// The next two should be admin paths, but not now,
		creategroupWebPagePath: state.creategroupWebpageHandler,
		deletegroupWebPagePath: state.deletegroupWebpageHandler,
	}

	//nonAdminWebPaths := []string{indexPath}
	cookie := testCreateValidCookie()

	for path, testFunc := range testWebEndpoints {
		req, err := http.NewRequest("GET", path, nil)
		if err != nil {
			t.Fatal(err)
		}
		//cookie := testCreateValidCookie()
		req.AddCookie(&cookie)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(testFunc)

		handler.ServeHTTP(rr, req)
		// Check the status code is what we expect.
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}
	}
	//now ensure unathenticated failed
	for path, testFunc := range testWebEndpoints {
		req, err := http.NewRequest("GET", path, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(testFunc)

		handler.ServeHTTP(rr, req)
		// Check the status code is what we expect.
		if status := rr.Code; status != http.StatusFound {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}
	}
}

func TestAdminOnlyWebPaths(t *testing.T) {
	state, err := setupTestState()
	if err != nil {
		log.Println(err)
	}
	testWebEndpoints := map[string]http.HandlerFunc{
		changeownershipPath:         state.changeownershipWebpageHandler,
		createServiceAccWebPagePath: state.createserviceAccountPageHandler,
	}

	adminCookie := testCreateValidAdminCookie()

	for path, testFunc := range testWebEndpoints {
		req, err := http.NewRequest("GET", path, nil)
		if err != nil {
			t.Fatal(err)
		}
		//cookie := testCreateValidCookie()
		req.AddCookie(&adminCookie)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(testFunc)

		handler.ServeHTTP(rr, req)
		// Check the status code is what we expect.
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}
	}

	cookie := testCreateValidCookie()

	for path, testFunc := range testWebEndpoints {
		req, err := http.NewRequest("GET", path, nil)
		if err != nil {
			t.Fatal(err)
		}
		//cookie := testCreateValidCookie()
		req.AddCookie(&cookie)

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(testFunc)

		handler.ServeHTTP(rr, req)
		// Check the status code is what we expect.
		if status := rr.Code; status != http.StatusForbidden {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusForbidden)
		}
	}
	//now ensure unathenticated failed
	for path, testFunc := range testWebEndpoints {
		req, err := http.NewRequest("GET", path, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(testFunc)

		handler.ServeHTTP(rr, req)
		// Check the status code is what we expect.
		if status := rr.Code; status != http.StatusFound {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}
	}
}

func TestRequestAccessHandler(t *testing.T) {
	state, err := setupTestState()
	if err != nil {
		log.Println(err)
	}
	//formValues := url.Values{"groupnames": {"group1"}, "managegroup": {"group1"}}
	//req, err := http.NewRequest("POST", changeownershipbuttonPath, strings.NewReader(formValues.Encode()))
	requestData := map[string][]string{
		"groups": []string{"group3"},
	}
	jsonBytes, err := json.Marshal(requestData)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest("POST", changeownershipbuttonPath, bytes.NewReader(jsonBytes))
	if err != nil {
		t.Fatal(err)
	}
	cookie := testCreateValidCookie() //testCreateValidAdminCookie()
	req.AddCookie(&cookie)
	//This is actually not neded
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(state.requestAccessHandler)

	handler.ServeHTTP(rr, req)
	// Check the status code is what we expect.
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	////Now we test the delete request

	delReq, err := http.NewRequest("POST", deleterequestsPath, bytes.NewReader(jsonBytes))
	if err != nil {
		t.Fatal(err)
	}
	delReq.AddCookie(&cookie)
	delReq.Header.Set("Content-Type", "application/json")

	rr2 := httptest.NewRecorder()
	handler2 := http.HandlerFunc(state.deleteRequests)

	handler2.ServeHTTP(rr2, delReq)
	// Check the status code is what we expect.
	if status := rr2.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}
}

func TestCreateUserorNot(t *testing.T) {
	state, err := setupTestState()
	if err != nil {
		t.Fatal(err)
	}
	err = state.createUserorNot("user1")
	if err != nil {
		t.Fatal(err)
	}
	err = state.createUserorNot("user1")
	if err != nil {
		t.Fatal(err)
	}
	/*
		err = state.createUserorNot("non-existing-user")
		if err != nil {
			t.Fatal(err)
		}*/
}
