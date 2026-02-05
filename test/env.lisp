(in-package :acc/test)

(fiveam:def-suite env)
(fiveam:in-suite env)

(fiveam:test test-env-returns
  (fiveam:is (eq :test-type (acc::find-return-type (acc::env-extend (acc::make-env :return-type :test-type))))))