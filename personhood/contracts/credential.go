package contracts

import (
	"go.dedis.ch/cothority/v3/byzcoin"
	"go.dedis.ch/cothority/v3/darc"
)

// Clone returns a copy of the credentials
func (c CredentialStruct) Clone() CredentialStruct {
	cs := CredentialStruct{}
	for _, cred := range c.Credentials {
		cs.Credentials = append(cs.Credentials, cred.Clone())
	}
	return cs
}

// Get returns the credential with the given name.
// If the name cannot be found, an empty Credential is returned.
func (c CredentialStruct) Get(name CredentialEntry) Credential {
	for _, cred := range c.Credentials {
		if cred.Name == name {
			return cred.Clone()
		}
	}
	return Credential{}
}

// GetPublic returns the public attribute
func (c CredentialStruct) GetPublic(name AttributePublic) []byte {
	return c.Get(CEPublic).Get(AttributeString(name)).Value
}

// GetConfig returns the config attribute
func (c CredentialStruct) GetConfig(name AttributeConfig) []byte {
	return c.Get(CEConfig).Get(AttributeString(name)).Value
}

// Set overwrites a given Credential or inserts it, if it doesn't exist yet.
func (c *CredentialStruct) Set(name CredentialEntry, attrs []Attribute) {
	for i, cred := range c.Credentials {
		if cred.Name == name {
			c.Credentials[i].Attributes = attrs
			return
		}
	}
	cred := Credential{
		Name:       name,
		Attributes: attrs,
	}
	c.Credentials = append(c.Credentials, cred)
}

// SetCred sets the credential. Take care when using this function, as it
// cannot check if the attribute and the credentialEntry match.
func (c *CredentialStruct) SetCred(ce CredentialEntry, attr AttributeString,
	value []byte) {
	cred := c.Get(ce)
	cred.Set(attr, value)
	c.Set(ce, cred.Attributes)
}

// SetPublic sets the public credential.
func (c *CredentialStruct) SetPublic(attr AttributePublic, value []byte) {
	c.SetCred(CEPublic, AttributeString(attr), value)
}

// SetConfig sets the configuration credential.
func (c *CredentialStruct) SetConfig(attr AttributeConfig, value []byte) {
	c.SetCred(CEConfig, AttributeString(attr), value)
}

// SetContacts sets the contacts of the user in the credential.
func (c *CredentialStruct) SetContacts(contacts []byzcoin.InstanceID) {
	iidLen := len(byzcoin.InstanceID{})
	cs := make([]byte, len(contacts)*iidLen)
	start := 0
	for _, id := range contacts {
		copy(cs[start:start+iidLen], id[:])
		start += iidLen
	}
	c.SetPublic(APContacts, cs)
}

// SetDevices sets the devices of the user in the credential.
func (c *CredentialStruct) SetDevices(devices map[string]darc.ID) {
	var devs []Attribute
	for name, dID := range devices {
		devs = append(devs, Attribute{
			Name:  AttributeString(name),
			Value: dID[:],
		})
	}
	c.Set(CEDevices, devs)
}

// GetDevices returns a copy of the devices in the credential.
func (c *CredentialStruct) GetDevices() map[string]darc.ID {
	devices := make(map[string]darc.ID)
	for _, attr := range c.Get(CEDevices).Attributes {
		devices[string(attr.Name)] = attr.Value
	}
	return devices
}

// SetRecoveries sets the Recoveries of the user in the credential.
func (c *CredentialStruct) SetRecoveries(recoveries map[string]byzcoin.InstanceID) {
	var rcs []Attribute
	for name, dID := range recoveries {
		rcs = append(rcs, Attribute{
			Name:  AttributeString(name),
			Value: dID[:],
		})
	}
	c.Set(CERecoveries, rcs)
}

// GetRecoveries returns a copy of the Recoveries in the credential.
func (c *CredentialStruct) GetRecoveries() map[string]byzcoin.InstanceID {
	recoveries := make(map[string]byzcoin.InstanceID)
	for _, attr := range c.Get(CERecoveries).Attributes {
		recoveries[string(attr.Name)] = byzcoin.NewInstanceID(attr.Value)
	}
	return recoveries
}

// Clone returns a deep copy of the Credential.
func (c Credential) Clone() Credential {
	newCred := Credential{Name: c.Name,
		Attributes: make([]Attribute, len(c.Attributes))}
	for i, attr := range c.Attributes {
		newCred.Attributes[i] = attr.Clone()
	}
	return newCred
}

// Get returns the Attribute with the given name.
// If no Attribute with that name is found, an empty Attribute is returned.
func (c Credential) Get(name AttributeString) Attribute {
	for _, attr := range c.Attributes {
		if attr.Name == name {
			return attr.Clone()
		}
	}
	return Attribute{Name: name}
}

// Set overwrites a given Attribute or inserts it, if it doesn't exist yet.
func (c *Credential) Set(name AttributeString, value []byte) {
	for i, attr := range c.Attributes {
		if attr.Name == name {
			c.Attributes[i] = Attribute{Name: name, Value: value}
			return
		}
	}
	c.Attributes = append(c.Attributes, Attribute{Name: name, Value: value})
}

// Clone returns a deep copy of the Attribute.
func (a Attribute) Clone() Attribute {
	v := make([]byte, len(a.Value))
	copy(v, a.Value)
	return Attribute{Name: a.Name, Value: v}
}
