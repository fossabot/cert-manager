/*
Copyright 2017 Jetstack Ltd.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package certificate

import (
	"flag"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
	cmutil "github.com/jetstack/cert-manager/pkg/util"
	"github.com/jetstack/cert-manager/test/e2e/framework"
	"github.com/jetstack/cert-manager/test/util"
)

const invalidACMEURL = "http://not-a-real-acme-url.com"
const testingACMEEmail = "test@example.com"
const testingACMEPrivateKey = "test-acme-private-key"
const foreverTestTimeout = time.Second * 60

var acmeIngressClass string

func init() {
	flag.StringVar(&acmeIngressClass, "acme-nginx-ingress-class", "nginx", ""+
		"The ingress class for the nginx ingress controller")
}

var _ = framework.CertManagerDescribe("ACME Certificate (HTTP01)", func() {
	f := framework.NewDefaultFramework("create-acme-certificate-http01")

	issuerName := "test-acme-issuer"
	certificateName := "test-acme-certificate"
	certificateSecretName := "test-acme-certificate"

	BeforeEach(func() {
		By("Verifying there is no existing ACME private key")
		_, err := f.KubeClientSet.CoreV1().Secrets(f.Namespace.Name).Get(testingACMEPrivateKey, metav1.GetOptions{})
		Expect(err).To(MatchError(apierrors.NewNotFound(corev1.Resource("secrets"), testingACMEPrivateKey)))
		By("Verifying there is no existing TLS certificate secret")
		_, err = f.KubeClientSet.CoreV1().Secrets(f.Namespace.Name).Get(certificateSecretName, metav1.GetOptions{})
		Expect(err).To(MatchError(apierrors.NewNotFound(corev1.Resource("secrets"), certificateSecretName)))
		By("Creating an Issuer")
		_, err = f.CertManagerClientSet.CertmanagerV1alpha1().Issuers(f.Namespace.Name).Create(util.NewCertManagerACMEIssuer(issuerName, framework.TestContext.ACMEURL, testingACMEEmail, testingACMEPrivateKey, 0, 0))
		Expect(err).NotTo(HaveOccurred())
		By("Waiting for Issuer to become Ready")
		err = util.WaitForIssuerCondition(f.CertManagerClientSet.CertmanagerV1alpha1().Issuers(f.Namespace.Name),
			issuerName,
			v1alpha1.IssuerCondition{
				Type:   v1alpha1.IssuerConditionReady,
				Status: v1alpha1.ConditionTrue,
			})
		Expect(err).NotTo(HaveOccurred())
		By("Verifying the ACME account URI is set")
		err = util.WaitForIssuerStatusFunc(f.CertManagerClientSet.CertmanagerV1alpha1().Issuers(f.Namespace.Name),
			issuerName,
			func(i *v1alpha1.Issuer) (bool, error) {
				if i.GetStatus().ACMEStatus().URI == "" {
					return false, nil
				}
				return true, nil
			})
		Expect(err).NotTo(HaveOccurred())
		By("Verifying ACME account private key exists")
		secret, err := f.KubeClientSet.CoreV1().Secrets(f.Namespace.Name).Get(testingACMEPrivateKey, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		if len(secret.Data) != 1 {
			Fail("Expected 1 key in ACME account private key secret, but there was %d", len(secret.Data))
		}
	})

	AfterEach(func() {
		By("Cleaning up")
		f.CertManagerClientSet.CertmanagerV1alpha1().Issuers(f.Namespace.Name).Delete(issuerName, nil)
		f.KubeClientSet.CoreV1().Secrets(f.Namespace.Name).Delete(testingACMEPrivateKey, nil)
	})

	It("should obtain a signed certificate with a single CN from the ACME server", func() {
		By("Creating a Certificate")
		cert, err := f.CertManagerClientSet.CertmanagerV1alpha1().Certificates(f.Namespace.Name).Create(util.NewCertManagerACMECertificate(certificateName, certificateSecretName, issuerName, v1alpha1.IssuerKind, acmeIngressClass, util.ACMECertificateDomain))
		Expect(err).NotTo(HaveOccurred())
		f.WaitCertificateIssuedValid(cert)
	})

	It("should obtain a signed certificate with a CN and single subdomain as dns name from the ACME server", func() {
		By("Creating a Certificate")
		cert, err := f.CertManagerClientSet.CertmanagerV1alpha1().Certificates(f.Namespace.Name).Create(util.NewCertManagerACMECertificate(certificateName, certificateSecretName, issuerName, v1alpha1.IssuerKind, acmeIngressClass, util.ACMECertificateDomain, fmt.Sprintf("%s.%s", cmutil.RandStringRunes(5), util.ACMECertificateDomain)))
		Expect(err).NotTo(HaveOccurred())
		f.WaitCertificateIssuedValid(cert)
	})

	It("should fail to obtain a certificate for an invalid ACME dns name", func() {
		By("Creating a Certificate")
		_, err := f.CertManagerClientSet.CertmanagerV1alpha1().Certificates(f.Namespace.Name).Create(util.NewCertManagerACMECertificate(certificateName, certificateSecretName, issuerName, v1alpha1.IssuerKind, acmeIngressClass, "google.com"))
		Expect(err).NotTo(HaveOccurred())
		By("Waiting for the Certificate to not have a ready condition")
		err = util.WaitForCertificateCondition(f.CertManagerClientSet.CertmanagerV1alpha1().Certificates(f.Namespace.Name),
			certificateName,
			v1alpha1.CertificateCondition{
				Type:   v1alpha1.CertificateConditionReady,
				Status: v1alpha1.ConditionTrue,
			}, foreverTestTimeout)
		Expect(err).To(HaveOccurred())
		By("Verifying TLS certificate secret does not exist")
		_, err = f.KubeClientSet.CoreV1().Secrets(f.Namespace.Name).Get(certificateSecretName, metav1.GetOptions{})
		Expect(err).To(MatchError(apierrors.NewNotFound(corev1.Resource("secrets"), certificateSecretName)))
	})

	cases := []struct {
		inputDuration    time.Duration
		inputRenewBefore time.Duration
		expectedDuration time.Duration
		label            string
		event            string
	}{
		{time.Hour * 24 * 365, 0, time.Hour * 24 * 90, "NEWT should obtain a valid certificate with duration of 90 days from ACME server configured with a TTL of 90 days when asking for a duration of 365 days", "WarnCertificateDuration"},
		{time.Hour * 24 * 240, time.Hour * 24 * 120, time.Hour * 24 * 90, "NEWT should obtain a valid certificate with a warning event when renewBefore is bigger than the duration", "WarnScheduleModified"},
	}

	for _, v := range cases {
		v := v
		It(v.label, func() {
			issuerName := "test-acme-issuer-duration"
			certificateName := "test-acme-certificate-duration"
			certificateSecretName := "test-acme-certificate-duration"

			By("Creating an Issuer")
			_, err := f.CertManagerClientSet.CertmanagerV1alpha1().Issuers(f.Namespace.Name).Create(util.NewCertManagerACMEIssuer(issuerName, framework.TestContext.ACMEURL, testingACMEEmail, testingACMEPrivateKey, v.inputDuration, v.inputRenewBefore))
			Expect(err).NotTo(HaveOccurred())

			By("Waiting for Issuer to become Ready")
			err = util.WaitForIssuerCondition(f.CertManagerClientSet.CertmanagerV1alpha1().Issuers(f.Namespace.Name),
				issuerName,
				v1alpha1.IssuerCondition{
					Type:   v1alpha1.IssuerConditionReady,
					Status: v1alpha1.ConditionTrue,
				})
			Expect(err).NotTo(HaveOccurred())

			By("Creating a Certificate")
			cert, err := f.CertManagerClientSet.CertmanagerV1alpha1().Certificates(f.Namespace.Name).Create(util.NewCertManagerACMECertificate(certificateName, certificateSecretName, issuerName, v1alpha1.IssuerKind, acmeIngressClass, util.ACMECertificateDomain, fmt.Sprintf("%s.%s", cmutil.RandStringRunes(5), util.ACMECertificateDomain)))
			Expect(err).NotTo(HaveOccurred())

			f.WaitCertificateIssuedValid(cert)
			f.CertificateDurationValid(cert, v.expectedDuration)

			f.WaitForCertificateEvent(cert, v.event)

			By("Cleaning up")
			f.CertManagerClientSet.CertmanagerV1alpha1().Issuers(f.Namespace.Name).Delete(issuerName, nil)
		})
	}
})
