// Package main provides ...
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

		Convey("When I request a token", func() {
			token, err = GetToken(clientId, secret)

			Convey("Then I should get a access-token", func() {
				So(token, ShouldNotBeNil)
				So(err, ShouldBeNil)
			})

			Convey("And I request a simple payment", func() {
				payment, err = CreatePayPalPayment(
					token,
					1.00, 0.20, 2.00, "USD",
					"Die Dinge die ich eingekauft habe.",
					"http://lillypark.com/ok", "http://lillypark.com/cancel")

				Convey("Then I should get a created payment ready for authorization", func() {
					So(err, ShouldBeNil)
					So(payment, ShouldNotBeNil)

					So(payment.State, ShouldEqual, "created")
					So(payment.Intent, ShouldEqual, "sale")
				})
			})
		})
	})
}
