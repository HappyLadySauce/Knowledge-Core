package options

import "errors"

func (o *Options) Validate() error {
	var err error
	err = errors.Join(err, o.InsecureServing.Validate())
	err = errors.Join(err, o.SQLite.Validate())
	err = errors.Join(err, o.JWT.Validate())
	err = errors.Join(err, o.Library.Validate())
	return err
}
