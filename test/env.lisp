(in-package :acc)

(fiveam:def-suite env)

(fiveam:test test-env-returns
  (fiveam:is (eq :test-type (find-return-type (env-extend (make-env :return-type :test-type))))))