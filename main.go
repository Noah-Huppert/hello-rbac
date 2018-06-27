package main

import "fmt"
import "strings"
import "strconv"

import "github.com/ory/ladon"
import "github.com/satori/go.uuid"
import manager "github.com/ory/ladon/manager/memory"

// Fake db data

// DBUser mocks a row in a user database table
type DBUser struct {
	ID   int
	Name string
}

// DBSquad mocks a row in a squad database table. A squad is a group which
// multiple users can access.
type DBSquad struct {
	ID   int
	Name string
}

// DBSquadMembership mocks a row in a squad membership table. Marks that a user
// is part of a squad.
type DBSquadMembership struct {
	ID      int
	SquadID int
	UserID  int
}

// users mocks a users database
var users = []DBUser{
	DBUser{0, "Foo"},
	DBUser{1, "Bar"},
	DBUser{2, "Bazz"},
}

// squads mocks a squad database
var squads = []DBSquad{
	DBSquad{0, "All Squad"},
	DBSquad{1, "Exclusive Squad"},
}

// squadMemberships mocks a squad membership database
var squadMemberships = []DBSquadMembership{
	DBSquadMembership{0, 0, 0}, // user:Foo -> squad:All Squad
	DBSquadMembership{1, 0, 1}, // user:Bar -> squad:All Squad
	DBSquadMembership{2, 0, 2}, // user:Bazz -> squad:All Squad
	DBSquadMembership{3, 1, 0}, // user:Foo -> squad:Exclusive Squad
	DBSquadMembership{4, 1, 1}, // user:Bar -> squad:Exclusive Squad
}

// Permission conditions
// extractURIID parses a URI for a resource ID.
func extract_uri_id(uri string) (int, error) {
	parts := strings.Split(uri, ":")

	if len(parts) != 3 {
		return -1, fmt.Errorf("uri does not contain 3 parts, "+
			"uri: \"%s\"", uri)
	}

	if parts[0] != "model" {
		return -1, fmt.Errorf("uri is not for a model, "+
			"uri: \"%s\"", uri)
	}

	id, err := strconv.Atoi(parts[2])
	if err != nil {
		return -1, fmt.Errorf("error converting third part of uri "+
			"to int, uri: \"%s\", error: %s", uri, err.Error())
	}

	return id, nil
}

// SelfCondition ensures that the subject of a request matches the resource
// being accessed.
type SelfCondition struct{}

func (c SelfCondition) GetName() string {
	return "SelfCondition"
}

func (c SelfCondition) Fulfills(_ interface{}, req *ladon.Request) bool {
	fmt.Printf("%s == %s\n", req.Subject, req.Resource)
	return req.Subject == req.Resource
}

// SquadMemberCondition ensures the subject is a member of the squad in a
// request.
type SquadMemberCondition struct{}

func (c SquadMemberCondition) GetName() string {
	return "SquadMemberCondition"
}

func (c SquadMemberCondition) Fulfills(_ interface{}, req *ladon.Request) bool {
	userID, err := extract_uri_id(req.Subject)
	if err != nil {
		fmt.Printf("SquadMemberCondition error extracting user id: %s\n",
			err.Error())
		return false
	}

	squadID, err := extract_uri_id(req.Resource)
	if err != nil {
		fmt.Printf("SquadMemberCondition error extracting squad id: %s\n",
			err.Error())
		return false
	}

	for _, membership := range squadMemberships {
		if membership.SquadID == squadID && membership.UserID == userID {
			return true
		}
	}

	return false
}

// test_request submits a request to a Ladon Warden and prints the result
func test_request(name string, warden *ladon.Ladon, req *ladon.Request,
	shouldFail bool) {

	fmt.Printf("Testing: \"%s\"\n", name)

	err := warden.IsAllowed(req)

	if shouldFail {
		if err == nil {
			fmt.Println("    BAD: Did not fail")
		} else {
			fmt.Printf("    OK: Failed with: \"%s\"\n", err.Error())
		}
	} else {
		if err == nil {
			fmt.Println("    OK")
		} else {
			fmt.Printf("    BAD: Failed with: \"%s\"\n", err.Error())
		}
	}
}

func main() {
	// Create warden
	warden := &ladon.Ladon{
		Manager: manager.NewMemoryManager(),
	}

	// Define policies
	policies := []ladon.DefaultPolicy{
		ladon.DefaultPolicy{
			ID:          uuid.NewV4().String(),
			Description: "Only user can update their profile",
			Subjects:    []string{"model:user:<.*>"},
			Resources:   []string{"model:user:<.*>"},
			Actions:     []string{"service:user:set"},
			Effect:      ladon.AllowAccess,
			Conditions: ladon.Conditions{
				"self": SelfCondition{},
			},
		},
		ladon.DefaultPolicy{
			ID:          uuid.NewV4().String(),
			Description: "Only squad members can view squad info",
			Subjects:    []string{"model:user:<.*>"},
			Resources:   []string{"model:squad:<.*>"},
			Actions:     []string{"service:squad:get"},
			Effect:      ladon.AllowAccess,
			Conditions: ladon.Conditions{
				"member": SquadMemberCondition{},
			},
		},
	}

	for _, policy := range policies {
		err := warden.Manager.Create(&policy)
		if err != nil {
			fmt.Printf("error creating \"%s\" policy: %s\n",
				policy.Description, err.Error())
			return
		}
		fmt.Printf("Loaded %s policy\n", policy.ID)
	}

	// Test access
	// TODO: Figure out why user policy tests break when squad policy was added
	test_request("self allowed service:user:set", warden, &ladon.Request{
		Subject:  "model:user:0",
		Action:   "service:user:set",
		Resource: "model:user:0",
		Context:  ladon.Context{},
	}, false)

	test_request("other not allowed service:user:set", warden, &ladon.Request{
		Subject:  "model:user:1",
		Action:   "service:user:set",
		Resource: "model:user:0",
		Context:  ladon.Context{},
	}, true)

	test_request("members allowed service:squad:get", warden, &ladon.Request{
		Subject:  "model:user:0",
		Action:   "service:squad:get",
		Resource: "model:squad:1",
		Context:  ladon.Context{},
	}, false)

	test_request("non members not allowed service:squad:get", warden, &ladon.Request{
		Subject:  "model:user:2",
		Action:   "service:squad:get",
		Resource: "model:squad:1",
		Context:  ladon.Context{},
	}, true)
}
