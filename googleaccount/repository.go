package googleaccount

import (
	"golang.org/x/net/context"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
)

func FetchByProviderID(ctx context.Context, id string) (GoogleAccount, error) {
	var ga GoogleAccount
	var gas []GoogleAccount

	query := datastore.NewQuery("GoogleAccount").Filter("ProviderID =", id).Order("UpdatedAt")
	_, err := query.GetAll(ctx, &gas)
	if err != nil {
		log.Debugf(ctx, "googleaccount.FetchByProviderID query failed. query=%+v, err=%+v", query, err)
		return ga, err
	}

	if len(gas) > 0 {
		return gas[0], nil
	}

	return ga, datastore.ErrNoSuchEntity
}
