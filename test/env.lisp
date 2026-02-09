(in-package :acc/test)

(fiveam:def-suite env)
(fiveam:in-suite env)

(fiveam:test env-find-return (fiveam:is (eq :test-type (acc::find-return-type (acc::env-extend (acc::make-env :return-type :test-type))))))

(fiveam:test env-find-return-err (fiveam:signals error (acc::find-return-type (acc::make-env))))

(fiveam:test
 env-find-symbol
 (let ((env (acc::make-env)))
   (fiveam:finishes (acc::register-symbol env "test" :int))
   (fiveam:is (eq :int (acc::env-symbol-sym-type (acc::find-env-symbol (acc::env-extend env) "test"))))))

(fiveam:test env-find-symbol-missing (fiveam:signals error (acc::find-env-symbol (acc::make-env) "test")))