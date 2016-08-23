// Package medco contains the code for the medco protocols.
// Medco stands for medical cothority and more precisely for privacy-preserving medical data sharing
// using a cothority. We use medical data and more precisely medical surveys as a working example
// but we intend to create a more general framework which would be a decentralized database containing
// any kind of data that could be queried in a privacy-preserving way.
//
// This medco package contains the protocols which permit to do a private survey.
// Once the servers (nodes) of the cothority have gathered the client responses encrypted with El-Gamal
// under the collective public key of the cothority (constructed with the secret of each node in order to have
// strongest-link security),
// the nodes can:
//	- transform an El-Gamal encrypted response into a Pohlig-Hellman (PH) encrypted vector. Since
// 	  Pohlig-Hellman is a deterministic encryption, it permits to compare (for equality) two
//	  encrypted ciphertexts (deterministic_switching_protocol)
//	- switch back from the deterministic PH encryption to an El-Gamal probabilistic encryption
//	  (probabilistic_switching_protocol)
//	- transform an El-Gamal ciphertext encrypted under one key to another key without decrypting it
//	  (key_switching_protocol)
//	- collectively aggregate their local results (private_aggregate_protocol)
package medco
