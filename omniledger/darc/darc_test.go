package darc

import (
	"encoding/json"
	"fmt"
	"testing"
	//	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/dedis/onet.v1/log"
)

func TestDarc(t *testing.T) {
	var rules []*Rule
	for i := 0; i < 2; i++ {
		rules = append(rules, createRule().rule)
	}
	d := NewDarc(&rules)
	for i, rule := range rules {
		require.Equal(t, *rule, *(*d.Rules)[i])
	}
}

func TestDarc_Copy(t *testing.T) {
	d1 := createDarc().darc
	d2 := d1.Copy()
	d1.Version = 3
	d1.AddRule(createRule().rule)
	require.NotEqual(t, len(*d1.Rules), len(*d2.Rules))
	require.NotEqual(t, d1.Version, d2.Version)
	d2 = d1.Copy()
	require.Equal(t, d1.GetID(), d2.GetID())
	require.Equal(t, len(*d1.Rules), len(*d2.Rules))
}

func TestDarc_AddRule(t *testing.T) {
	d := createDarc().darc
	rule := createRule().rule
	d.AddRule(rule)
	require.Equal(t, rule, (*d.Rules)[len(*d.Rules)-1])
}

func TestDarc_RemoveRule(t *testing.T) {
	d1 := createDarc().darc
	d2 := d1.Copy()
	rule := createRule().rule
	d2.AddRule(rule)
	require.NotEqual(t, len(*d1.Rules), len(*d2.Rules))
	d2.RemoveRule(len(*d2.Rules) - 1)
	require.Equal(t, len(*d1.Rules), len(*d2.Rules))
}

func TestDarc_RuleUpdateAction(t *testing.T) {
	d1 := createDarc().darc
	rule := createRule().rule
	d1.AddRule(rule)
	d2 := d1.Copy()
	ind1 := len(*d1.Rules) - 1
	ind2 := len(*d2.Rules) - 1
	require.Equal(t, (*d1.Rules)[ind1].Action, (*d2.Rules)[ind2].Action)
	action := string("TestUpdate")
	d2.RuleUpdateAction(ind2, action)
	require.NotEqual(t, (*d1.Rules)[ind1].Action, (*d2.Rules)[ind2].Action)
}

func TestDarc_RuleAddSubject(t *testing.T) {
	d := createDarc().darc
	s := createSubject_PK()
	d.RuleAddSubject(0, s)
	ind := len(*(*d.Rules)[0].Subjects) - 1
	r1 := (*d.Rules)[0]
	s1 := (*r1.Subjects)[ind]
	require.Equal(t, s, s1)
}

func TestDarc_RuleRemoveSubject(t *testing.T) {
	d1 := createDarc().darc
	d2 := d1.Copy()
	s := createSubject_PK()
	d2.RuleAddSubject(0, s)
	require.NotEqual(t, len(*(*d1.Rules)[0].Subjects), len(*(*d2.Rules)[0].Subjects))
	d2.RuleRemoveSubject(0, s)
	require.Equal(t, len(*(*d1.Rules)[0].Subjects), len(*(*d2.Rules)[0].Subjects))
}

func TestDarc_RuleUpdateExpression(t *testing.T) {
	d1 := createDarc().darc
	rule := createRule().rule
	d1.AddRule(rule)
	d2 := d1.Copy()
	ind := len(*d2.Rules) - 1
	require.Equal(t, (*d1.Rules)[ind].Expression, (*d2.Rules)[ind].Expression)
	d2.RuleUpdateExpression(ind, `{"or" : [0,1]}`)
	require.NotEqual(t, (*d1.Rules)[ind].Expression, (*d2.Rules)[ind].Expression)
}

func TestRequest_Copy(t *testing.T) {
	req, _ := createRequest()
	req1 := req.request
	req2 := req1.CopyReq()
	req1.RuleID = 1000
	require.NotEqual(t, req1.RuleID, req2.RuleID)
	require.Equal(t, req1.DarcID, req2.DarcID)
	req2 = req1.CopyReq()
	require.Equal(t, req1.RuleID, req2.RuleID)
}

func TestRequest_Sign(t *testing.T) {
	r, signer := createRequest()
	req := r.request
	_, err := signer.Sign(req)
	if err != nil {
		fmt.Println(err)
	}
	//fmt.Println("Signature:", sig.Signature)
}

func TestRequest_SignWithPathCheck(t *testing.T) {
	r, signer := createRequestMultiPath()
	req := r.request
	_, pa, err := signer.SignWithPathCheck(req, darcMap)
	if err != nil {
		fmt.Println(err)
		if pa != nil {
			fmt.Println(pa)
		}
	}
}

func TestRequest_Verify(t *testing.T) {
	req, signer := createRequest2()
	sig, err := signer.Sign(req.request)
	if err != nil {
		log.ErrFatal(err)
	}
	err = Verify(req.request, sig, darcMap)
	if err != nil {
		fmt.Println(err)
	} else {
		var raw interface{}
		json.Unmarshal(req.request.Message, &raw)
		fmt.Println("Single-sig Verification works")
	}
}

func TestRequest_VerifySigWithPath(t *testing.T) {
	r, signer := createRequestMultiPath()
	req := r.request
	_, pa, err := signer.SignWithPathCheck(req, darcMap)
	if err != nil {
		fmt.Println(err)
		if pa != nil {
			//fmt.Println(pa[0])
			sig, err := signer.SignWithPath(req, pa[0])
			if err != nil {
				fmt.Println(err)
			}
			err = VerifySigWithPath(req, sig, darcMap)
			if err != nil {
				fmt.Println(err)
			} else {
				var raw interface{}
				json.Unmarshal(req.Message, &raw)
				fmt.Println("Single-sig Verification with path works")
			}
		}
	}
}

func TestRequestMultiSig_Verify(t *testing.T) {
	req, signers := createRequestMultiSig()
	var signatures []*Signature
	for _, signer := range signers {
		sig, err := signer.Sign(req.request)
		if err != nil {
			log.ErrFatal(err)
		}
		signatures = append(signatures, sig)
	}
	err := VerifyMultiSig(req.request, signatures, darcMap)
	if err != nil {
		fmt.Println(err)
	} else {
		var raw interface{}
		json.Unmarshal(req.request.Message, &raw)
		fmt.Println("Multi-sig Verification works")
	}
}

func TestDarc_IncrementVersion(t *testing.T) {
	d := createDarc().darc
	previousVersion := d.Version
	d.IncrementVersion()
	require.NotEqual(t, previousVersion, d.Version)
}

func benchmarkRequest_SignWithPathCheck(total int, depth int, b *testing.B) {
	req, signer := createRequestMultiPathAtDepth(total, depth)
	multipaths := 0
	for n := 0; n < b.N; n++ {
		_, pa, err := signer.SignWithPathCheck(req.request, darcMap)
		if err != nil {
			//fmt.Println(err)
			if pa != nil {
				//fmt.Println(pa)
				multipaths += 1
			}
		}
	}
	//fmt.Println(multipaths)
}

//func BenchmarkRequest_SignWithPathCheck2_2(b *testing.B)  { benchmarkRequest_SignWithPathCheck(2, 2, b) }
// func BenchmarkRequest_SignWithPathCheck5_2(b *testing.B)  { benchmarkRequest_SignWithPathCheck(5, 2, b) }
// func BenchmarkRequest_SignWithPathCheck10_2(b *testing.B)  { benchmarkRequest_SignWithPathCheck(10, 2, b) }
// func BenchmarkRequest_SignWithPathCheck20_2(b *testing.B)  { benchmarkRequest_SignWithPathCheck(20, 2, b) }
// func BenchmarkRequest_SignWithPathCheck50_2(b *testing.B)  { benchmarkRequest_SignWithPathCheck(50, 2, b) }
// func BenchmarkRequest_SignWithPathCheck100_2(b *testing.B)  { benchmarkRequest_SignWithPathCheck(100, 2, b) }

// func BenchmarkRequest_SignWithPathCheck2_10(b *testing.B)  { benchmarkRequest_SignWithPathCheck(2, 10, b) }
// func BenchmarkRequest_SignWithPathCheck5_10(b *testing.B)  { benchmarkRequest_SignWithPathCheck(5, 10, b) }
// func BenchmarkRequest_SignWithPathCheck10_10(b *testing.B)  { benchmarkRequest_SignWithPathCheck(10, 10, b) }
// func BenchmarkRequest_SignWithPathCheck20_10(b *testing.B)  { benchmarkRequest_SignWithPathCheck(20, 10, b) }
// func BenchmarkRequest_SignWithPathCheck50_10(b *testing.B)  { benchmarkRequest_SignWithPathCheck(50, 10, b) }
// func BenchmarkRequest_SignWithPathCheck100_10(b *testing.B)  { benchmarkRequest_SignWithPathCheck(100, 10, b) }

func benchmarkRequest_Verify(depth int, b *testing.B) {
	req, signer := createRequestAtDepth(depth)
	sig, err := signer.Sign(req.request)
	if err != nil {
		log.ErrFatal(err)
	}
	failed := 0
	for n := 0; n < b.N; n++ {
		err = Verify(req.request, sig, darcMap)
		if err != nil {
			failed += 1
		}
	}
}

// func BenchmarkRequest_Verify1(b *testing.B)  { benchmarkRequest_Verify(1, b) }
// func BenchmarkRequest_Verify2(b *testing.B)  { benchmarkRequest_Verify(2, b) }
// func BenchmarkRequest_Verify5(b *testing.B)  { benchmarkRequest_Verify(5, b) }
// func BenchmarkRequest_Verify10(b *testing.B)  { benchmarkRequest_Verify(10, b) }
// func BenchmarkRequest_Verify20(b *testing.B)  { benchmarkRequest_Verify(20, b) }
// func BenchmarkRequest_Verify50(b *testing.B)  { benchmarkRequest_Verify(50, b) }
// func BenchmarkRequest_Verify100(b *testing.B)  { benchmarkRequest_Verify(100, b) }

// func TestRequestMultiSig_VerifyAtDepth(t *testing.T) {
// 	req, signers  := createRequestMultiSigAtDepthNum(2, 1)
// 	var signatures []*Signature
// 	for _, signer := range signers {
// 		sig, err := signer.Sign(req.request)
// 		if err != nil {
// 			log.ErrFatal(err)
// 		}
// 		signatures = append(signatures, sig)
// 	}
// 	err := VerifyMultiSig(req.request, signatures, darcMap)
// 	if err != nil {
// 		fmt.Println(err)
// 	} else {
// 		var raw interface{}
//     	json.Unmarshal(req.request.Message, &raw)
// 		fmt.Println("Multi-sig Verification works")
// 	}
// }

func benchmarkRequest_VerifySigWithPath(total int, depth int, b *testing.B) {
	req, signer := createRequestMultiPathAtDepth(total, depth)
	_, pa, err := signer.SignWithPathCheck(req.request, darcMap)
	fail := 0
	if err != nil {
		//fmt.Println(err)
		if pa != nil {
			//fmt.Println(pa[0])
			sig, err := signer.SignWithPath(req.request, pa[0])
			if err != nil {
				fmt.Println(err)
			}
			for n := 0; n < b.N; n++ {
				err = VerifySigWithPath(req.request, sig, darcMap)
				if err != nil {
					fail += 1
					//fmt.Println(err)
				}
			}
		}
	}
	//fmt.Println("Fail", fail)
}

// func BenchmarkRequest_VerifySigWithPath2_2(b *testing.B)  { benchmarkRequest_VerifySigWithPath(2, 2, b) }
// func BenchmarkRequest_VerifySigWithPath5_2(b *testing.B)  { benchmarkRequest_VerifySigWithPath(5, 2, b) }
// func BenchmarkRequest_VerifySigWithPath10_2(b *testing.B)  { benchmarkRequest_VerifySigWithPath(10, 2, b) }
// func BenchmarkRequest_VerifySigWithPath20_2(b *testing.B)  { benchmarkRequest_VerifySigWithPath(20, 2, b) }
// func BenchmarkRequest_VerifySigWithPath50_2(b *testing.B)  { benchmarkRequest_VerifySigWithPath(50, 2, b) }
// func BenchmarkRequest_VerifySigWithPath100_2(b *testing.B)  { benchmarkRequest_VerifySigWithPath(100, 2, b) }

func BenchmarkRequest_VerifySigWithPath2_10(b *testing.B) {
	benchmarkRequest_VerifySigWithPath(2, 10, b)
}
func BenchmarkRequest_VerifySigWithPath5_10(b *testing.B) {
	benchmarkRequest_VerifySigWithPath(5, 10, b)
}
func BenchmarkRequest_VerifySigWithPath10_10(b *testing.B) {
	benchmarkRequest_VerifySigWithPath(10, 10, b)
}
func BenchmarkRequest_VerifySigWithPath20_10(b *testing.B) {
	benchmarkRequest_VerifySigWithPath(20, 10, b)
}
func BenchmarkRequest_VerifySigWithPath50_10(b *testing.B) {
	benchmarkRequest_VerifySigWithPath(50, 10, b)
}
func BenchmarkRequest_VerifySigWithPath100_10(b *testing.B) {
	benchmarkRequest_VerifySigWithPath(100, 10, b)
}

func benchmarkRequestMultiSig_Verify(numsigs int, depth int, b *testing.B) {
	req, signers := createRequestMultiSigAtDepthNum(numsigs, depth)
	var signatures []*Signature
	for _, signer := range signers {
		sig, err := signer.Sign(req.request)
		if err != nil {
			log.ErrFatal(err)
		}
		signatures = append(signatures, sig)
	}
	//failed := 0
	for n := 0; n < b.N; n++ {
		_ = VerifyMultiSig(req.request, signatures, darcMap)
		// if err != nil {
		// 	failed += 1
		// }
	}
	//fmt.Println(failed)
}

// func BenchmarkRequestMultiSig_Verify2_2(b *testing.B)  { benchmarkRequestMultiSig_Verify(2, 2, b) }
// func BenchmarkRequestMultiSig_Verify5_2(b *testing.B)  { benchmarkRequestMultiSig_Verify(5, 2, b) }
// func BenchmarkRequestMultiSig_Verify10_2(b *testing.B)  { benchmarkRequestMultiSig_Verify(10, 2, b) }
// func BenchmarkRequestMultiSig_Verify20_2(b *testing.B)  { benchmarkRequestMultiSig_Verify(20, 2, b) }
// func BenchmarkRequestMultiSig_Verify50_2(b *testing.B)  { benchmarkRequestMultiSig_Verify(50, 2, b) }
// func BenchmarkRequestMultiSig_Verify100_2(b *testing.B)  { benchmarkRequestMultiSig_Verify(100, 2, b) }

// func BenchmarkRequestMultiSig_Verify2_10(b *testing.B)  { benchmarkRequestMultiSig_Verify(2, 10, b) }
// func BenchmarkRequestMultiSig_Verify5_10(b *testing.B)  { benchmarkRequestMultiSig_Verify(5, 10, b) }
// func BenchmarkRequestMultiSig_Verify10_10(b *testing.B)  { benchmarkRequestMultiSig_Verify(10, 10, b) }
// func BenchmarkRequestMultiSig_Verify20_10(b *testing.B)  { benchmarkRequestMultiSig_Verify(20, 10, b) }
// func BenchmarkRequestMultiSig_Verify50_10(b *testing.B)  { benchmarkRequestMultiSig_Verify(50, 10, b) }
// func BenchmarkRequestMultiSig_Verify100_10(b *testing.B)  { benchmarkRequestMultiSig_Verify(100, 10, b) }

var darcMap = make(map[string]*Darc)

type testDarc struct {
	darc  *Darc
	rules []*Rule
}

type testRule struct {
	rule     *Rule
	subjects []*Subject
}

type testRequest struct {
	request *Request
}

func createDarc() *testDarc {
	td := &testDarc{}
	r := createAdminRule()
	td.rules = append(td.rules, r.rule)
	r = createUserRule()
	td.rules = append(td.rules, r.rule)
	td.darc = NewDarc(&td.rules)
	darcMap[string(td.darc.GetID())] = td.darc
	return td
}

func createAdminRule() *testRule {
	tr := &testRule{}
	action := "Admin"
	expression := `{"and" : [0, 1]}`
	for i := 0; i < 3; i++ {
		s := createSubject_PK()
		tr.subjects = append(tr.subjects, s)
	}
	tr.rule = &Rule{Action: action, Subjects: &tr.subjects, Expression: expression}
	return tr
}

func createUserRule() *testRule {
	tr := &testRule{}
	action := "User"
	expression := `{"and" : [0, 1]}`
	for i := 0; i < 2; i++ {
		s := createSubject_PK()
		tr.subjects = append(tr.subjects, s)
	}
	tr.rule = &Rule{Action: action, Subjects: &tr.subjects, Expression: expression}
	return tr
}

func createRule() *testRule {
	tr := &testRule{}
	action := "Read"
	expression := `{}`
	s1 := createSubject_PK()
	s2 := createSubject_Darc()
	tr.subjects = append(tr.subjects, s1)
	tr.subjects = append(tr.subjects, s2)
	tr.rule = &Rule{Action: action, Subjects: &tr.subjects, Expression: expression}
	return tr
}

func createSubject_Darc() *Subject {
	rule := createAdminRule().rule
	var rules []*Rule
	rules = append(rules, rule)
	rule = createUserRule().rule
	rules = append(rules, rule)
	darc := NewDarc(&rules)
	id := darc.GetID()
	darcMap[string(id)] = darc
	subjectdarc := NewSubjectDarc(id)
	subject, _ := NewSubject(subjectdarc, nil)
	return subject
}

func createSubject_PK() *Subject {
	_, subject := createSignerSubject()
	return subject
}

func createSigner() *Signer {
	signer, _ := createSignerSubject()
	return signer
}

func createSignerSubject() (*Signer, *Subject) {
	edSigner := NewEd25519Signer(nil, nil)
	signer := &Signer{Ed25519: edSigner}
	subjectpk := NewSubjectPK(signer.Ed25519.Point)
	subject, err := NewSubject(nil, subjectpk)
	log.ErrFatal(err)
	return signer, subject
}

func createRequest() (*testRequest, *Signer) {
	tr := &testRequest{}
	dr := createDarc().darc
	dr_id := dr.GetID()
	sig, sub := createSignerSubject()
	dr.RuleAddSubject(1, sub)
	msg, _ := json.Marshal("Document1")
	request := NewRequest(dr_id, 0, msg)
	tr.request = request
	return tr, sig
}

func createRequest2() (*testRequest, *Signer) {
	tr := &testRequest{}
	dr := createDarc().darc
	dr_id := dr.GetID()
	sub1 := createSubject_Darc()
	dr.RuleAddSubject(0, sub1)
	dr2 := darcMap[string(sub1.Darc.ID)]
	sig, sub := createSignerSubject()
	dr2.RuleAddSubject(1, sub)
	msg, _ := json.Marshal("Document1")
	request := NewRequest(dr_id, 0, msg)
	tr.request = request
	return tr, sig
}

func createRequestAtDepth(depth int) (*testRequest, *Signer) {
	tr := &testRequest{}
	dr := createDarc().darc
	cur_darc := dr
	dr_id := dr.GetID()
	for i := 0; i < depth; i++ {
		sub1 := createSubject_Darc()
		cur_darc.RuleAddSubject(1, sub1)
		cur_darc = darcMap[string(sub1.Darc.ID)]
	}
	sig, sub := createSignerSubject()
	cur_darc.RuleAddSubject(1, sub)
	msg, _ := json.Marshal("Document1")
	request := NewRequest(dr_id, 1, msg)
	tr.request = request
	return tr, sig
}

func createRequestMultiPathAtDepth(total int, depth int) (*testRequest, *Signer) {
	tr := &testRequest{}
	dr := createDarc().darc
	dr_id := dr.GetID()
	cur_darc := dr
	for i := 0; i < total*2; i++ {
		sub := createSubject_Darc()
		cur_darc.RuleAddSubject(1, sub)
	}
	rule := (*dr.Rules)[1]
	subs := *rule.Subjects
	sig, subject := createSignerSubject()
	for i := 2; i < total*2; i += 2 {
		sub2 := subs[i]
		sub_darc := *sub2.Darc
		cur_darc := darcMap[string(sub_darc.ID)]
		for j := 0; j < depth; j++ {
			sub1 := createSubject_Darc()
			cur_darc.RuleAddSubject(1, sub1)
			cur_darc = darcMap[string(sub1.Darc.ID)]
		}
		cur_darc.RuleAddSubject(1, subject)
	}
	msg, _ := json.Marshal("Document1")
	request := NewRequest(dr_id, 1, msg)
	tr.request = request
	return tr, sig
}

func createRequestMultiSigAtDepthNum(numsigs int, depth int) (*testRequest, []*Signer) {
	tr := &testRequest{}
	dr := createDarc().darc
	dr_id := dr.GetID()
	var signers []*Signer
	for i := 0; i < numsigs; i++ {
		cur_darc := dr
		for j := 0; j < depth; j++ {
			sub1 := createSubject_Darc()
			cur_darc.RuleAddSubject(1, sub1)
			cur_darc = darcMap[string(sub1.Darc.ID)]
		}
		sig, sub := createSignerSubject()
		cur_darc.RuleAddSubject(1, sub)
		signers = append(signers, sig)
	}
	dr.RuleUpdateExpression(1, `{"and" : [2, 3]}`)
	msg, _ := json.Marshal("Document1")
	request := NewRequest(dr_id, 1, msg)
	tr.request = request
	return tr, signers
}

func createRequestMultiSig() (*testRequest, []*Signer) {
	tr := &testRequest{}
	dr := createDarc().darc
	dr_id := dr.GetID()
	var signers []*Signer
	for i := 0; i < 2; i++ {
		sig, sub := createSignerSubject()
		dr.RuleAddSubject(0, sub)
		signers = append(signers, sig)
	}
	dr.RuleUpdateExpression(0, `{"and" : [3, 4]}`)
	msg, _ := json.Marshal(createDarc().darc)
	request := NewRequest(dr_id, 0, msg)
	tr.request = request
	return tr, signers
}

func createRequestMultiPath() (*testRequest, *Signer) {
	tr := &testRequest{}
	dr_id, sig := createMultiPathSubject()
	msg, _ := json.Marshal(createDarc().darc)
	request := NewRequest(dr_id, 1, msg)
	tr.request = request
	return tr, sig
}

func createMultiPathSubject() ([]byte, *Signer) {
	dedis := createDarc().darc
	dr_id := dedis.GetID()
	research := createSubject_Darc()
	software := createSubject_Darc()
	engineering := createSubject_Darc()
	sandra := createSubject_Darc()
	sig, linus := createSignerSubject()
	dedis.RuleAddSubject(1, research)
	dedis.RuleAddSubject(1, software)
	dedis.RuleAddSubject(1, engineering)
	dedis.RuleAddSubject(1, linus)
	rsub := darcMap[string(research.Darc.ID)]
	rsub.RuleAddSubject(1, sandra)
	sandrasub := darcMap[string(sandra.Darc.ID)]
	sandrasub.RuleAddSubject(1, linus)
	swsub := darcMap[string(software.Darc.ID)]
	swsub.RuleAddSubject(1, linus)
	engsub := darcMap[string(engineering.Darc.ID)]
	engsub.RuleAddSubject(1, linus)
	return dr_id, sig
}
