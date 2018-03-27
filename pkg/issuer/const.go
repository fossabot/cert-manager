package issuer

import (
	"fmt"
	"time"

	"github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha1"
)

const (
	// Default to 30 days if Issuer.spec.renewBefore is not set
	RenewCertificateBefore = time.Hour * 24 * 30
	// Minimum duration before renewal.
	MinimumCertificateValidityDuration = time.Minute * 5
	// Minium certificate duration
	MinimumCertificateDuration = RenewCertificateBefore + MinimumCertificateValidityDuration

	// IssuerACME is the name of the ACME issuer
	IssuerACME string = "acme"
	// IssuerCA is the name of the simple issuer
	IssuerCA string = "ca"
)

// nameForIssuer determines the name of the issuer implementation given an
// Issuer resource.
func nameForIssuer(i v1alpha1.GenericIssuer) (string, error) {
	switch {
	case i.GetSpec().ACME != nil:
		return IssuerACME, nil
	case i.GetSpec().CA != nil:
		return IssuerCA, nil
	}
	return "", fmt.Errorf("no issuer specified for Issuer '%s/%s'", i.GetObjectMeta().Namespace, i.GetObjectMeta().Name)
}
