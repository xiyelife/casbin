// Copyright 2017 EDOMO Systems GmbH. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sessionrolemanager

import (
	"sort"

	"github.com/casbin/casbin/rbac"
	"github.com/casbin/casbin/util"
)

type RoleManager struct {
	allRoles          map[string]*SessionRole
	maxHierarchyLevel int
}

// SessionRoleManager provides an implementation for the RoleManagerConstructor that
// supports RBAC sessions with a start time and an end time.
func SessionRoleManager() rbac.RoleManagerConstructor {
	return func() rbac.RoleManager {
		return NewRoleManager(10)
	}
}

// NewRoleManager is the constructor for creating an instance of the
// SessionRoleManager implementation.
func NewRoleManager(maxHierarchyLevel int) rbac.RoleManager {
	rm := RoleManager{}
	rm.allRoles = make(map[string]*SessionRole)
	rm.maxHierarchyLevel = maxHierarchyLevel
	return &rm
}

func (rm *RoleManager) hasRole(name string) bool {
	_, ok := rm.allRoles[name]
	return ok
}

func (rm *RoleManager) createRole(name string) *SessionRole {
	if !rm.hasRole(name) {
		rm.allRoles[name] = newSessionRole(name)
	}
	return rm.allRoles[name]
}

// Clear clears all stored data and resets the role manager to the initial state.
func (rm *RoleManager) Clear() {
	rm.allRoles = make(map[string]*SessionRole)
}

// AddLink adds the inheritance link between role: name1 and role: name2.
// aka role: name1 inherits role: name2.
// timeRange is the time range when the role inheritance link is active.
func (rm *RoleManager) AddLink(name1 string, name2 string, timeRange ...string) {
	if len(timeRange) != 2 {
		return
	}
	startTime := timeRange[0]
	endTime := timeRange[1]

	role1 := rm.createRole(name1)
	role2 := rm.createRole(name2)

	session := Session{role2, startTime, endTime}
	role1.addSession(session)
}

// DeleteLink deletes the inheritance link between role: name1 and role: name2.
// aka role: name1 does not inherit role: name2 any more.
// unused is not used.
func (rm *RoleManager) DeleteLink(name1 string, name2 string, unused ...string) {
	if !rm.hasRole(name1) || !rm.hasRole(name2) {
		return
	}

	role1 := rm.createRole(name1)
	role2 := rm.createRole(name2)

	role1.deleteSessions(role2.name)
}

// HasLink determines whether role: name1 inherits role: name2.
// requestTime is the querying time for the role inheritance link.
func (rm *RoleManager) HasLink(name1 string, name2 string, requestTime ...string) bool {
	if len(requestTime) != 1 {
		return false
	}

	if name1 == name2 {
		return true
	}

	if !rm.hasRole(name1) || !rm.hasRole(name2) {
		return false
	}

	role1 := rm.createRole(name1)
	return role1.hasValidSession(name2, rm.maxHierarchyLevel, requestTime[0])
}

// GetRoles gets the roles that a subject inherits.
// currentTime is the querying time for the role inheritance link.
func (rm *RoleManager) GetRoles(name string, currentTime ...string) []string {
	if len(currentTime) != 1 {
		return nil
	}
	requestTime := currentTime[0]

	if !rm.hasRole(name) {
		return nil
	}

	sessionRoles := rm.createRole(name).getSessionRoles(requestTime)
	return sessionRoles
}

// GetUsers gets the users that inherits a subject.
// currentTime is the querying time for the role inheritance link.
func (rm *RoleManager) GetUsers(name string, currentTime ...string) []string {
	if len(currentTime) != 1 {
		return nil
	}
	requestTime := currentTime[0]

	users := []string{}
	for _, role := range rm.allRoles {
		if role.hasDirectRole(name, requestTime) {
			users = append(users, role.name)
		}
	}
	sort.Strings(users)
	return users
}

// PrintRoles prints all the roles to log.
func (rm *RoleManager) PrintRoles() {
	for _, role := range rm.allRoles {
		util.LogPrint(role.toString())
	}
}

// SessionRole is a modified version of the default role.
// A SessionRole not only has a name, but also a list of sessions.
type SessionRole struct {
	name     string
	sessions []Session
}

func newSessionRole(name string) *SessionRole {
	sr := SessionRole{name: name}
	return &sr
}

func (sr *SessionRole) addSession(s Session) {
	sr.sessions = append(sr.sessions, s)
}

func (sr *SessionRole) deleteSessions(sessionName string) {
	// Delete sessions from an array while iterating it
	index := 0
	for _, srs := range sr.sessions {
		if srs.role.name != sessionName {
			sr.sessions[index] = srs
			index++
		}
	}
	sr.sessions = sr.sessions[:index]
}

//
//func (sr *SessionRole) getSessions() []Session {
//	return sr.sessions
//}

func (sr *SessionRole) getSessionRoles(requestTime string) []string {
	names := []string{}
	for _, session := range sr.sessions {
		if session.startTime <= requestTime && requestTime <= session.endTime {
			if !contains(names, session.role.name) {
				names = append(names, session.role.name)
			}
		}
	}
	return names
}

func (sr *SessionRole) hasValidSession(name string, hierarchyLevel int, requestTime string) bool {
	if hierarchyLevel == 1 {
		return sr.name == name
	}

	for _, s := range sr.sessions {
		if s.startTime <= requestTime && requestTime <= s.endTime {
			if s.role.name == name {
				return true
			}
			if s.role.hasValidSession(name, hierarchyLevel-1, requestTime) {
				return true
			}
		}
	}
	return false
}

func (sr *SessionRole) hasDirectRole(name string, requestTime string) bool {
	for _, session := range sr.sessions {
		if session.role.name == name {
			if session.startTime <= requestTime && requestTime <= session.endTime {
				return true
			}
		}
	}
	return false
}

func (sr *SessionRole) toString() string {
	sessions := ""
	for i, session := range sr.sessions {
		if i == 0 {
			sessions += session.role.name
		} else {
			sessions += ", " + session.role.name
		}
		sessions += " (until: " + session.endTime + ")"
	}
	return sr.name + " < " + sessions
}

// Session represents the activation of a role inheritance for a
// specified time. A role inheritance is always bound to its temporal validity.
// As soon as a session loses its validity, the corresponding role inheritance
// becomes invalid too.
type Session struct {
	role      *SessionRole
	startTime string
	endTime   string
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
