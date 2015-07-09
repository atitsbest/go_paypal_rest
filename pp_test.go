
package main

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestSpec(t *testing.T) {
	Convey("Given some paypal credentials (ENV)", t, func() {
		clientId, secret := fetchEnvVars(t)
		var err error
		var token string
		var payment *PaymentResponse

		Convey("When I request a token...", func() {
			token, err = GetToken(clientId, secret)

			Convey("then I should get an access-token", func() {
				So(token, ShouldNotBeNil)
				So(err, ShouldBeNil)
			})

			Convey("When I request a simple payment...", func() {
				payment, err = CreatePayPalPayment(
					token,
					1.00, 0.20, 2.00, "USD",
					"The products that I have purchased:",
					"http://lillypark.com/ok", "http://lillypark.com/cancel")

				Convey("then I should get a created payment which is ready for authorization", func() {
					So(err, ShouldBeNil)
					So(payment, ShouldNotBeNil)

					So(payment.State, ShouldEqual, "created")
					So(payment.Intent, ShouldEqual, "sale")
				})
			})
		})
	})
}
