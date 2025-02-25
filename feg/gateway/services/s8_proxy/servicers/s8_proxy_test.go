/*
Copyright 2020 The Magma Authors.
This source code is licensed under the BSD-style license found in the
LICENSE file in the root directory of this source tree.
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package servicers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"magma/feg/cloud/go/protos"
	"magma/feg/gateway/gtp"
	"magma/feg/gateway/services/s8_proxy/servicers/mock_pgw"

	"github.com/stretchr/testify/assert"
	"github.com/wmnsk/go-gtp/gtpv2"
	"github.com/wmnsk/go-gtp/gtpv2/ie"
)

const (
	GtpTimeoutForTest = gtp.DefaultGtpTimeout // use the same the default value defined in s8_proxy
	//port 0 means golang will choose the port. Selected port will be injected on getDefaultConfig
	s8proxyAddrs = ":0" // equivalent to sgwAddrs
	pgwAddrs     = "127.0.0.1:0"
	IMSI1        = "123456789012345"
	BEARER       = 5
	AGWTeidU     = uint32(10)
	AGWTeidC     = uint32(2)
	PDNType      = protos.PDNType_IPV4
	PAA          = "10.0.0.10"
)

func TestS8proxyCreateAndDeleteSession(t *testing.T) {
	// set up client ans server
	s8p, mockPgw := startSgwAndPgw(t, GtpTimeoutForTest)
	defer mockPgw.Close()

	// ------------------------
	// ---- Create Session ----
	csReq := getDefaultCreateSessionRequest(mockPgw.LocalAddr().String())

	// force PGW to return specific control plane PGW TEID
	PgwTEIDc := uint32(111)
	mockPgw.CreateSessionOptions.PgwTEIDc = PgwTEIDc

	// Send and receive Create Session Request
	csRes, err := s8p.CreateSession(context.Background(), csReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, csRes)
	assert.Empty(t, csRes.GtpError)

	// check User Plane FTEID was received properly
	assert.NotNil(t, csRes.BearerContext)
	assert.Equal(t, mockPgw.LastTEIDu, csRes.BearerContext.UserPlaneFteid.Teid)
	assert.NotEmpty(t, csRes.BearerContext.UserPlaneFteid.Ipv4Address)
	assert.Empty(t, csRes.BearerContext.UserPlaneFteid.Ipv6Address)

	// check Agw control Plane TEID on the response
	assert.Equal(t, AGWTeidC, csRes.CAgwTeid)

	// check Pgw Control Plane TEID
	assert.NotEmpty(t, csRes.CPgwFteid)
	assert.Equal(t, PgwTEIDc, csRes.CPgwFteid.Teid)

	// check PAA and PDN Allocation
	assert.Equal(t, PDNType, csRes.PdnType)
	assert.Equal(t, PAA, csRes.Paa.Ipv4Address)
	assert.Equal(t, "", csRes.Paa.Ipv6Address)

	// check QOS received at PGW
	sentQos := csReq.BearerContext.Qos
	receivedAtPGWQos := mockPgw.LastQos
	assert.Equal(t, sentQos.Gbr.BrDl, receivedAtPGWQos.Gbr.BrDl)
	assert.Equal(t, sentQos.Gbr.BrUl, receivedAtPGWQos.Gbr.BrUl)
	assert.Equal(t, sentQos.Mbr.BrDl, receivedAtPGWQos.Mbr.BrDl)
	assert.Equal(t, sentQos.Mbr.BrUl, receivedAtPGWQos.Mbr.BrUl)
	assert.Equal(t, sentQos.Qci, receivedAtPGWQos.Qci)

	// check QOS received at Response (should be the same as the sent)
	assert.NotEmpty(t, csRes.BearerContext.Qos)
	receivedQOS := csRes.BearerContext.Qos

	assert.True(t, csRes.BearerContext.ValidQos)
	assert.Equal(t, sentQos.Gbr.BrDl, receivedQOS.Gbr.BrDl)
	assert.Equal(t, sentQos.Gbr.BrUl, receivedQOS.Gbr.BrUl)
	assert.Equal(t, sentQos.Mbr.BrDl, receivedQOS.Mbr.BrDl)
	assert.Equal(t, sentQos.Mbr.BrUl, receivedQOS.Mbr.BrUl)
	assert.Equal(t, sentQos.Qci, receivedQOS.Qci)

	// check PCO
	assert.NotEmpty(t, csRes.ProtocolConfigurationOptions)
	assert.Equal(t, csReq.ProtocolConfigurationOptions, csRes.ProtocolConfigurationOptions)

	// ------------------------
	// ---- Delete Session ----
	cdReq := getDeleteSessionRequest(mockPgw.LocalAddr().String(), csRes.CPgwFteid.Teid)

	dsRes, err := s8p.DeleteSession(context.Background(), cdReq)
	assert.NoError(t, err)
	assert.Empty(t, dsRes.GtpError)
	// session shouldnt exist anymore
	_, err = s8p.gtpClient.GetSessionByIMSI(IMSI1)
	assert.Error(t, err)

	// disable option
	mockPgw.CreateSessionOptions.PgwTEIDc = 0

	// Create again the same session
	csRes, err = s8p.CreateSession(context.Background(), csReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, csRes)
}

func TestS8proxyRepeatedCreateSession(t *testing.T) {
	// set up client ans server
	s8p, mockPgw := startSgwAndPgw(t, GtpTimeoutForTest)
	defer mockPgw.Close()

	// ------------------------
	// ---- Create Session ----
	csReq := getDefaultCreateSessionRequest(mockPgw.LocalAddr().String())

	// force PGW to return specific control plane PGW TEID
	PgwTEIDc := uint32(111)
	mockPgw.CreateSessionOptions.PgwTEIDc = PgwTEIDc

	// Send and receive Create Session Request
	csRes, err := s8p.CreateSession(context.Background(), csReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, csRes)
	assert.Empty(t, csRes.GtpError)

	// check Agw control Plane TEID on the response
	assert.Equal(t, AGWTeidC, csRes.CAgwTeid)

	// check Pgw Control Plane TEID
	assert.Equal(t, PgwTEIDc, csRes.CPgwFteid.Teid)

	// ------------------------
	// -Create Session (again)-
	// Create session with different tunnel ids
	PgwTEIDc += 1
	mockPgw.CreateSessionOptions.PgwTEIDc = PgwTEIDc
	csReq.CAgwTeid += 1

	// Send and receive Create Session Request
	csRes, err = s8p.CreateSession(context.Background(), csReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, csRes)
	assert.Empty(t, csRes.GtpError)

	// check Agw control Plane TEID on the response
	assert.Equal(t, csReq.CAgwTeid, csRes.CAgwTeid)

	// check Pgw Control Plane TEID
	assert.Equal(t, PgwTEIDc, csRes.CPgwFteid.Teid)
}

func TestS8proxyCreateWithMissingParam(t *testing.T) {
	// set up client ans server
	s8p, mockPgw := startSgwAndPgw(t, GtpTimeoutForTest)
	defer mockPgw.Close()

	// ------------------------
	// ---- Create Session ----
	csReq := getDefaultCreateSessionRequest(mockPgw.LocalAddr().String())
	csReq.BearerContext = nil

	// Send and receive Create Session Request
	_, err := s8p.CreateSession(context.Background(), csReq)
	assert.Error(t, err)
}

// TestS8ProxyDeleteSessionAfterClientRestars test if s8_proxy is able to handle an already
// created session after s8 has been restarted.
func TestS8ProxyDeleteSessionAfterClientRestars(t *testing.T) {
	// set up client ans server
	s8p, mockPgw := startSgwAndPgw(t, time.Second*600)
	defer mockPgw.Close()

	// ------------------------
	// ---- Create Session ----
	csReq := getDefaultCreateSessionRequest(mockPgw.LocalAddr().String())

	// Send and receive Create Session Request
	csRes, err := s8p.CreateSession(context.Background(), csReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, csRes)
	assert.Empty(t, csRes.GtpError)

	// ------------------------
	// --- Restart s8_proxy ---
	config := getDefaultConfig(mockPgw.LocalAddr().String(), time.Second*600)
	// grab the actual client address since it needs to be the same
	actualS8Address := strings.Replace(s8p.gtpClient.LocalAddr().String(), "[::]", "", -1)
	config.ClientAddr = actualS8Address
	// create again the client (simulate a restart)
	s8p.gtpClient.Close()
	// wait to make sure port is finally closed by kernel
	waitUntilPortIsFree()
	s8p, err = NewS8Proxy(config)
	if err != nil {
		t.Fatalf("Error creating S8 proxy +%s", err)
	}

	// ------------------------
	// ---- Delete Session ----
	dsReq := getDeleteSessionRequest(mockPgw.LocalAddr().String(), csRes.CPgwFteid.Teid)

	// session should be deleted
	dsRes, err := s8p.DeleteSession(context.Background(), dsReq)
	assert.NoError(t, err)
	assert.Empty(t, dsRes.GtpError)
	// session shouldnt exist anymore
	_, err = s8p.gtpClient.GetSessionByIMSI(IMSI1)
	assert.Error(t, err)
}

func TestS8ProxyDeleteInexistentSession(t *testing.T) {
	s8p, mockPgw := startSgwAndPgw(t, 200*time.Millisecond)
	defer mockPgw.Close()

	// ------------------------
	// ---- Delete Session inexistent session ----
	dsReq := &protos.DeleteSessionRequestPgw{Imsi: "000000000000015"}
	dsReq = &protos.DeleteSessionRequestPgw{
		PgwAddrs: mockPgw.LocalAddr().String(),
		Imsi:     "000000000000015",
		BearerId: 4,
		CAgwTeid: 88,
		CPgwTeid: 87,
		ServingNetwork: &protos.ServingNetwork{
			Mcc: "222",
			Mnc: "333",
		},
		Uli: &protos.UserLocationInformation{
			Lac:    1,
			Ci:     2,
			Sac:    3,
			Rac:    4,
			Tac:    5,
			Eci:    6,
			MeNbi:  7,
			EMeNbi: 8,
		},
	}
	_, err := s8p.DeleteSession(context.Background(), dsReq)
	assert.Error(t, err)
	assert.Equal(t, mockPgw.LastTEIDc, uint32(87))
}

func TestS8ProxyDeleteWithMissingParamaters(t *testing.T) {
	s8p, mockPgw := startSgwAndPgw(t, 200*time.Millisecond)
	defer mockPgw.Close()

	// ------------------------
	// ---- Delete Session inexistent session ----
	// create a bad create session request
	dsReq := getDeleteSessionRequest(mockPgw.LocalAddr().String(), 10)
	dsReq.Uli = nil
	_, err := s8p.DeleteSession(context.Background(), dsReq)
	assert.Error(t, err)
}

func TestS8proxyCreateSessionWithServiceDenial(t *testing.T) {
	// set up client ans server
	s8p, mockPgw := startSgwAndPgw(t, GtpTimeoutForTest)
	defer mockPgw.Close()

	// ------------------------
	// ---- Create Session ----
	csReq := getDefaultCreateSessionRequest(mockPgw.LocalAddr().String())

	// PGW denies service
	mockPgw.SetCreateSessionWithErrorCause(gtpv2.CauseServiceDenied)
	csRes, err := s8p.CreateSession(context.Background(), csReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, csRes)
	assert.NotEmpty(t, csRes.GtpError)
	assert.Equal(t, gtpv2.CauseServiceDenied, uint8(csRes.GtpError.Cause))
}

func TestS8proxyCreateSessionWithMissingIEonResponse(t *testing.T) {
	// set up client ans server
	s8p, mockPgw := startSgwAndPgw(t, GtpTimeoutForTest)
	defer mockPgw.Close()

	// ------------------------
	// ---- Create Session ----
	csReq := getDefaultCreateSessionRequest(mockPgw.LocalAddr().String())

	// s8_proxy forces a missing IE
	mockPgw.SetCreateSessionResponseWithMissingIE()
	csRes, err := s8p.CreateSession(context.Background(), csReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, csRes)
	assert.NotEmpty(t, csRes.GtpError)
	assert.Equal(t, gtpv2.CauseMandatoryIEMissing, uint8(csRes.GtpError.Cause))
	// check the error code is FullyQualifiedTEID

	assert.Contains(t, csRes.GtpError.Msg, strconv.FormatUint(uint64(ie.FullyQualifiedTEID), 10))
}

func TestS8proxyCreateSessionWithMissingIEMessage(t *testing.T) {
	// set up client ans server
	s8p, mockPgw := startSgwAndPgw(t, GtpTimeoutForTest)
	defer mockPgw.Close()

	// ------------------------
	// ---- Create Session ----
	csReq := getDefaultCreateSessionRequest(mockPgw.LocalAddr().String())

	// s8_proxy forces a missing IE
	missingIe := ie.New(ie.BearerContext, 0, nil)
	mockPgw.SetCreateSessionRequestWithMissingIE(missingIe)
	csRes, err := s8p.CreateSession(context.Background(), csReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, csRes)
	assert.NotEmpty(t, csRes.GtpError)
	assert.Equal(t, gtpv2.CauseMandatoryIEMissing, uint8(csRes.GtpError.Cause))

	// check log meesage contains the name of the missing ie
	assert.Contains(t, csRes.GtpError.Msg, missingIe.Name())
}

func TestS8proxyValidateCreateSession(t *testing.T) {
	// set up client ans server
	s8p, mockPgw := startSgwAndPgw(t, GtpTimeoutForTest)
	defer mockPgw.Close()

	// ------------------------
	// ---- Create Session ----
	csReq := getDefaultCreateSessionRequest(mockPgw.LocalAddr().String())

	// force error with missing bearer context
	csReq.BearerContext = &protos.BearerContext{}

	csRes, err := s8p.CreateSession(context.Background(), csReq)
	assert.Error(t, err)
	assert.Empty(t, csRes)
}

func TestS8proxyManyCreateAndDeleteSession(t *testing.T) {
	// set up client ans server
	s8p, mockPgw := startSgwAndPgw(t, 5*time.Second)
	defer mockPgw.Close()

	// ------------------------
	// ---- Create Sessions ----
	nRequest := 100
	pgwActualAddrs := mockPgw.LocalAddr().String()
	csReqs := getMultipleCreateSessionRequest(nRequest, pgwActualAddrs)

	// routines will write on specific index
	csResps := make([]*protos.CreateSessionResponsePgw, nRequest)
	errs := make(chan error, len(csReqs))
	// PGW denies service
	for i, csReq := range csReqs {
		csReqShadow := csReq
		index := i
		go func() {
			// we should report as an error either if there is a grpc issue or a gtp issue
			var errCSR error
			csResps[index], errCSR = s8p.CreateSession(context.Background(), csReqShadow)
			if errCSR != nil {
				errs <- fmt.Errorf("GRPC error during CreatSessionRequest %s", errCSR)
				return
			}

			if csResps[index].GtpError != nil {
				errs <- fmt.Errorf("GTP error during CreatSession %s", csResps[index].GtpError.Msg)
				return
			}
			errs <- nil
		}()
	}
	// wait for all create session to complete
	for i := 0; i < len(csReqs); i++ {
		err := <-errs
		if err != nil {
			t.Fatal(fmt.Errorf("Error Creating Sessions: %s", err))
		}
	}

	// Check no gtpClient sessions were left created
	for _, csReq := range csReqs {
		_, err := s8p.gtpClient.GetSessionByIMSI(csReq.Imsi)
		if err == nil {
			t.Fatal(fmt.Errorf(
				"Found a session that should have been deleted after Create Session, %s", csReq.Imsi))
		}
	}

	// ------------------------
	// ---- Delete Sessions ----
	errs = make(chan error, len(csReqs))
	for i, csReq := range csReqs {
		csReqShadow := csReq
		csResShadow := csResps[i]
		go func() {
			cdReq := &protos.DeleteSessionRequestPgw{
				PgwAddrs: pgwActualAddrs,
				Imsi:     csReqShadow.Imsi,
				BearerId: csResShadow.BearerContext.Id,
				CAgwTeid: csResShadow.CAgwTeid,
				CPgwTeid: csResShadow.CPgwFteid.Teid,
				ServingNetwork: &protos.ServingNetwork{
					Mcc: "222",
					Mnc: "333",
				},
				Uli: &protos.UserLocationInformation{
					Lac:    1,
					Ci:     2,
					Sac:    3,
					Rac:    4,
					Tac:    5,
					Eci:    6,
					MeNbi:  7,
					EMeNbi: 8,
				},
			}

			var errDSR error
			dsResps, errDSR := s8p.DeleteSession(context.Background(), cdReq)
			if errDSR != nil {
				errs <- fmt.Errorf("GRPC error during DeleteSession %s", errDSR)
				return
			}
			if dsResps.GtpError != nil {
				errs <- fmt.Errorf("GTP error during DeleteSession %s", dsResps.GtpError.Msg)
				return
			}
			errs <- nil
		}()
	}
	// wait for all delete request to complete
	for i := 0; i < len(csReqs); i++ {
		err := <-errs
		if err != nil {
			t.Fatal(fmt.Errorf("Error Deleting Sessions: %s", err))
		}
	}

	// check sessions are deleted
	for _, csReq := range csReqs {
		_, err := s8p.gtpClient.GetSessionByIMSI(csReq.Imsi)
		if err == nil {
			t.Fatal(fmt.Errorf(
				"Found a session that should have been deleted after Delete Session, %s", csReq.Imsi))
		}
	}
}

// TestS8proxyCreateSessionWrongSgwTEIDcFromPgw creates the situation where the PGW responds to the
// proper sequence message but with wrong SgwTeidC
func TestS8proxyCreateSessionWrongSgwTEIDcFromPgw(t *testing.T) {
	// set up client ans server
	// this test will timeout, reducing  gtp timeout to prevent waiting too much
	s8p, mockPgw := startSgwAndPgw(t, 200*time.Millisecond)
	defer mockPgw.Close()

	// ------------------------
	// ---- Create Session ----
	csReq := getDefaultCreateSessionRequest(mockPgw.LocalAddr().String())

	// PGW denies service
	mockPgw.CreateSessionOptions.SgwTEIDc = 99
	csRes, err := s8p.CreateSession(context.Background(), csReq)
	assert.Error(t, err)
	assert.Empty(t, csRes)
}

func TestS8proxyCreateSessionIPV6(t *testing.T) {
	// set up client ans server
	s8p, mockPgw := startSgwAndPgw(t, GtpTimeoutForTest)
	defer mockPgw.Close()

	// ------------------------
	// ---- Create Session ----
	csReq := getDefaultCreateSessionRequest(mockPgw.LocalAddr().String())

	// change IPv4 address for IPV6
	csReq.PdnType = protos.PDNType_IPV6
	ipv6Address := "2001:db8::8a2e:370:7334"
	csReq.Paa = &protos.PdnAddressAllocation{
		Ipv6Address: ipv6Address,
		Ipv6Prefix:  0,
	}

	// Send and receive Create Session Request
	csRes, err := s8p.CreateSession(context.Background(), csReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, csRes)

	// check PAA and PDN Allocation for ipv6
	assert.Equal(t, protos.PDNType_IPV6, csRes.PdnType)
	assert.Equal(t, "", csRes.Paa.Ipv4Address)
	assert.Equal(t, ipv6Address, csRes.Paa.Ipv6Address)
}

func TestS8proxyCreateSessionNillPAA(t *testing.T) {
	// set up client ans server
	s8p, mockPgw := startSgwAndPgw(t, GtpTimeoutForTest)
	defer mockPgw.Close()

	// ------------------------
	// ---- Create Session  IPv4----
	csReq := getDefaultCreateSessionRequest(mockPgw.LocalAddr().String())
	csReq.Paa = nil

	// Send and receive Create Session Request
	csRes, err := s8p.CreateSession(context.Background(), csReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, csRes)
	assert.Empty(t, csRes.GtpError)

	session, err := mockPgw.GetSessionByIMSI(csReq.Imsi)
	assert.NoError(t, err)
	// PGW should return us a valid IP, but this is not implemented on our
	// mock PGW so 0.0.0.0 will be good enough
	assert.Equal(t, "0.0.0.0", session.GetDefaultBearer().SubscriberIP)

	cdReq := getDeleteSessionRequest(mockPgw.LocalAddr().String(), csRes.CPgwFteid.Teid)
	dsRes, err := s8p.DeleteSession(context.Background(), cdReq)
	assert.NoError(t, err)
	assert.Empty(t, dsRes.GtpError)

	// ------------------------
	// ---- Create Session  IPv6----
	csReq = getDefaultCreateSessionRequest(mockPgw.LocalAddr().String())
	csReq.Paa = nil
	csReq.PdnType = protos.PDNType_IPV6

	// Send and receive Create Session Request
	csRes, err = s8p.CreateSession(context.Background(), csReq)
	assert.NoError(t, err)
	assert.NotEmpty(t, csRes)
	assert.Empty(t, csRes.GtpError)

	session, err = mockPgw.GetSessionByIMSI(csReq.Imsi)
	assert.NoError(t, err)
	assert.Equal(t, "::", session.GetDefaultBearer().SubscriberIP)

	cdReq = getDeleteSessionRequest(mockPgw.LocalAddr().String(), csRes.CPgwFteid.Teid)
	dsRes, err = s8p.DeleteSession(context.Background(), cdReq)
	assert.NoError(t, err)
	assert.Empty(t, dsRes.GtpError)
}

func TestS8proxyCreateSessionNoProtocolConfigurationOptions(t *testing.T) {
	// set up client ans server
	s8p, mockPgw := startSgwAndPgw(t, GtpTimeoutForTest)
	defer mockPgw.Close()

	// Test empty list of PCO containers
	// ------------------------
	// ---- Create Session ----
	csReq := getDefaultCreateSessionRequest(mockPgw.LocalAddr().String())
	csReq.ProtocolConfigurationOptions.ProtoOrContainerId = nil

	// Send and receive Create Session Request
	csRes, err := s8p.CreateSession(context.Background(), csReq)

	// check PCO
	assert.NoError(t, err)
	assert.NotEmpty(t, csRes.ProtocolConfigurationOptions)
	assert.Equal(t, csReq.ProtocolConfigurationOptions, csRes.ProtocolConfigurationOptions)

	// Test no PCO at all
	// ------------------------
	// ---- Delete Session ----
	cdReq := getDeleteSessionRequest(mockPgw.LocalAddr().String(), csRes.CPgwFteid.Teid)
	_, err = s8p.DeleteSession(context.Background(), cdReq)
	assert.NoError(t, err)

	// ------------------------
	// ---- Create Session ----
	csReq = getDefaultCreateSessionRequest(mockPgw.LocalAddr().String())
	csReq.ProtocolConfigurationOptions.IsValid = false
	csRes, err = s8p.CreateSession(context.Background(), csReq)

	// check PCO
	assert.NoError(t, err)
	assert.Nil(t, csRes.ProtocolConfigurationOptions)
}

func TestS8proxyEcho(t *testing.T) {
	s8p, mockPgw := startSgwAndPgw(t, GtpTimeoutForTest)
	defer mockPgw.Close()

	//------------------------
	//---- Echo Request ----
	eReq := &protos.EchoRequest{PgwAddrs: mockPgw.LocalAddr().String()}
	_, err := s8p.SendEcho(context.Background(), eReq)
	assert.NoError(t, err)
}

// startSgwAndPgw starts s8_proxy and a mock pgw for testing
func startSgwAndPgw(t *testing.T, gtpTimeout time.Duration) (*S8Proxy, *mock_pgw.MockPgw) {
	// Create and run PGW
	mockPgw, err := mock_pgw.NewStarted(nil, pgwAddrs)
	if err != nil {
		t.Fatalf("Error creating mock PGW: +%s", err)
	}

	// in case pgwAddres has a 0 port, mock_pgw will chose the port. With this variable we make
	// sure we use the right address (this only happens in testing)
	actualPgwAddress := mockPgw.LocalAddr().String()
	fmt.Printf("Running PGW at %s\n", actualPgwAddress)

	// Run S8_proxy
	config := getDefaultConfig(mockPgw.LocalAddr().String(), gtpTimeout)
	s8p, err := NewS8Proxy(config)
	if err != nil {
		t.Fatalf("Error creating S8 proxy +%s", err)
	}
	return s8p, mockPgw
}

func getDefaultCreateSessionRequest(pgwAddrs string) *protos.CreateSessionRequestPgw {
	_, offset := time.Now().Zone()
	return &protos.CreateSessionRequestPgw{
		PgwAddrs: pgwAddrs,
		Imsi:     IMSI1,
		Msisdn:   "300000000000003",
		Mei:      "111",
		CAgwTeid: AGWTeidC,
		ServingNetwork: &protos.ServingNetwork{
			Mcc: "222",
			Mnc: "333",
		},
		RatType: protos.RATType_EUTRAN,
		BearerContext: &protos.BearerContext{
			Id: BEARER,
			UserPlaneFteid: &protos.Fteid{
				Ipv4Address: "127.0.0.10",
				Ipv6Address: "",
				Teid:        AGWTeidU,
			},
			Qos: &protos.QosInformation{
				Pci:                     0,
				PriorityLevel:           0,
				PreemptionCapability:    0,
				PreemptionVulnerability: 0,
				Qci:                     9,
				Gbr: &protos.Ambr{
					BrUl: 123,
					BrDl: 234,
				},
				Mbr: &protos.Ambr{
					BrUl: 567,
					BrDl: 890,
				},
			},
		},
		PdnType: protos.PDNType_IPV4,
		Paa: &protos.PdnAddressAllocation{
			Ipv4Address: "10.0.0.10",
			Ipv6Address: "",
			Ipv6Prefix:  0,
		},

		Apn:           "internet.com",
		SelectionMode: protos.SelectionModeType_APN_provided_subscription_verified,
		Ambr: &protos.Ambr{
			BrUl: 999,
			BrDl: 888,
		},
		Uli: &protos.UserLocationInformation{
			Lac:    1,
			Ci:     2,
			Sac:    3,
			Rac:    4,
			Tac:    5,
			Eci:    6,
			MeNbi:  7,
			EMeNbi: 8,
		},
		ProtocolConfigurationOptions: &protos.ProtocolConfigurationOptions{
			IsValid:        true,
			ConfigProtocol: uint32(gtpv2.ConfigProtocolPPPWithIP),
			ProtoOrContainerId: []*protos.PcoProtocolOrContainerId{
				{
					Id:       uint32(gtpv2.ProtoIDIPCP),
					Length:   16, // len not required, just added to compare with the result which includes length
					Contents: []byte{0x01, 0x00, 0x00, 0x10, 0x03, 0x06, 0x01, 0x01, 0x01, 0x01, 0x81, 0x06, 0x02, 0x02, 0x02, 0x02},
				},
				{
					Id:       uint32(gtpv2.ProtoIDPAP),
					Length:   12, // len not required, just added to compare with the result which includes length
					Contents: []byte{0x01, 0x00, 0x00, 0x0c, 0x03, 0x66, 0x6f, 0x6f, 0x03, 0x62, 0x61, 0x72},
				},
				{
					Id:       uint32(gtpv2.ContIDMSSupportOfNetworkRequestedBearerControlIndicator),
					Length:   0, // len not required, just added to compare with the result which includes length
					Contents: nil,
				},
			},
		},
		IndicationFlag: nil,
		TimeZone: &protos.TimeZone{
			DeltaSeconds:       int32(offset),
			DaylightSavingTime: 0,
		},
	}
}

func getMultipleCreateSessionRequest(nRequest int, pgwAddrs string) []*protos.CreateSessionRequestPgw {
	res := []*protos.CreateSessionRequestPgw{}
	_, offset := time.Now().Zone()
	for i := 0; i < nRequest; i++ {
		newReq := &protos.CreateSessionRequestPgw{
			PgwAddrs: pgwAddrs,
			Imsi:     fmt.Sprintf("%d", 100000000000000+i),
			Msisdn:   fmt.Sprintf("%d", 17730000000+i),
			Mei:      fmt.Sprintf("%d", 900000000000000+i),
			CAgwTeid: AGWTeidC + uint32(i),
			ServingNetwork: &protos.ServingNetwork{
				Mcc: "222",
				Mnc: "333",
			},
			RatType: protos.RATType_EUTRAN,
			BearerContext: &protos.BearerContext{
				Id: BEARER,
				UserPlaneFteid: &protos.Fteid{
					Ipv4Address: "127.0.0.10",
					Ipv6Address: "",
					Teid:        AGWTeidU + uint32(i),
				},
				Qos: &protos.QosInformation{
					Pci:                     0,
					PriorityLevel:           0,
					PreemptionCapability:    0,
					PreemptionVulnerability: 0,
					Qci:                     9,
					Gbr: &protos.Ambr{
						BrUl: 123,
						BrDl: 234,
					},
					Mbr: &protos.Ambr{
						BrUl: 567,
						BrDl: 890,
					},
				},
			},
			PdnType: PDNType,
			Paa: &protos.PdnAddressAllocation{
				Ipv4Address: PAA,
				Ipv6Address: "",
				Ipv6Prefix:  0,
			},

			Apn:           "internet.com",
			SelectionMode: protos.SelectionModeType_APN_provided_subscription_verified,
			Ambr: &protos.Ambr{
				BrUl: 999,
				BrDl: 888,
			},
			ProtocolConfigurationOptions: &protos.ProtocolConfigurationOptions{
				IsValid:        true,
				ConfigProtocol: uint32(gtpv2.ConfigProtocolPPPWithIP),
				ProtoOrContainerId: []*protos.PcoProtocolOrContainerId{
					{
						Id:       uint32(gtpv2.ProtoIDIPCP),
						Contents: []byte{0x01, 0x00, 0x00, 0x10, 0x03, 0x06, 0x01, 0x01, 0x01, 0x01, 0x81, 0x06, 0x02, 0x02, 0x02, 0x02},
					},
					{
						Id:       uint32(gtpv2.ProtoIDPAP),
						Contents: []byte{0x01, 0x00, 0x00, 0x0c, 0x03, 0x66, 0x6f, 0x6f, 0x03, 0x62, 0x61, 0x72},
					},
					{
						Id:       uint32(gtpv2.ContIDMSSupportOfNetworkRequestedBearerControlIndicator),
						Contents: nil,
					},
				},
			},
			Uli: &protos.UserLocationInformation{
				Lac:    1,
				Ci:     2,
				Sac:    3,
				Rac:    4,
				Tac:    5,
				Eci:    6,
				MeNbi:  7,
				EMeNbi: 8,
			},
			IndicationFlag: nil,
			TimeZone: &protos.TimeZone{
				DeltaSeconds:       int32(offset),
				DaylightSavingTime: 0,
			},
		}
		res = append(res, newReq)
	}
	return res
}

func getDeleteSessionRequest(pgwAddrs string, cPgwTeid uint32) *protos.DeleteSessionRequestPgw {
	res := &protos.DeleteSessionRequestPgw{
		PgwAddrs: pgwAddrs,
		Imsi:     IMSI1,
		BearerId: BEARER,
		CAgwTeid: AGWTeidC,
		CPgwTeid: cPgwTeid,
		ServingNetwork: &protos.ServingNetwork{
			Mcc: "222",
			Mnc: "333",
		},
		Uli: &protos.UserLocationInformation{
			Lac:    1,
			Ci:     2,
			Sac:    3,
			Rac:    4,
			Tac:    5,
			Eci:    6,
			MeNbi:  7,
			EMeNbi: 8,
		},
	}
	return res
}

func getDefaultConfig(pgwActualAddrs string, gtpTimeout time.Duration) *S8ProxyConfig {
	return &S8ProxyConfig{
		GtpTimeout: gtpTimeout,
		ClientAddr: s8proxyAddrs,
	}
}

func waitUntilPortIsFree() {
	timeout := 20 * time.Millisecond
	for i := 0; i < 10; i++ {
		time.Sleep(timeout)
	}
}
