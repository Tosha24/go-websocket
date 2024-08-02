package actions

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type OTP struct {
	Key     string
	Created time.Time
}

type RetentionMap map[string]OTP

func NewRetentionMap(ctx context.Context, retentionPeriod time.Duration) RetentionMap {
	rm := make(RetentionMap) 

	go rm.Retention(ctx, retentionPeriod)  // rm is now linked with a method called Retention() that runs in a goroutine and continously checks for expired OTPs and deletes them from the map
	return rm
}

// NewOTP() -> creates a new OTP and adds it to the RetentionMap and return the current OTP.. it is called when a new client is connected to the server (login)
func (rm RetentionMap) NewOTP() OTP {
	// create a new OTP with unique key and current time
	o := OTP{
		Key: uuid.NewString(),
		Created: time.Now(),
	}

	rm[o.Key] = o // Adds the new OTP to the RetentionMap
	return o
}

// VerifyOTP() -> Verifies if an OTP exists and removes it from the map if it does
func (rm RetentionMap) VerifyOTP(otp string) bool {
	if _, ok := rm[otp]; !ok {
		return false
	}
	delete(rm, otp)
	return true
}

// Retention is a function that periodically checks and removes expired OTPs from the RetentionMap
// ctx is context to manage timeout and cancellation of the function
// retentionPeriod is the duration after which the OTPs should be cleaned up from the RetentionMap

func (rm RetentionMap) Retention(ctx context.Context, retentionPeriod time.Duration) {
	// ticker is a channel that creates ticker that ticks every 400 milliseconds
	ticker := time.NewTicker(400* time.Millisecond)

	// continuously check if the otps are expired and delete them from the map if they are.
	for {
		select {
		case <- ticker.C:  // <- ticker.C waits for the ticker to tick and then executes the code block	
			for _, otp := range rm { // iterate over RetentionMap for OTPs
				if otp.Created.Add((retentionPeriod)).Before(time.Now()) { // checks if the OTP is older than retentionPeriod, if yes, delete the otp from the map
					delete(rm, otp.Key)
				}
			}
		case <- ctx.Done():
			// exit the loop if the context is done (like timeout or cancellation or completion)
			return
		}
	}
}