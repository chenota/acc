(in-package :acc)

(fiveam:def-suite static-type)

(fiveam:test test-program
  (fiveam:is (set-program-types (parse-program (make-token-sequence (tokenize "func main int { return 0; }"))))))

(fiveam:test test-function
  (fiveam:is (null (function-type-parameters (assign-type (function-rule (make-token-sequence (tokenize "func main int { return 0; }"))) (make-env)))))
  (fiveam:is (integer-type-p (function-type-return-type (assign-type (function-rule (make-token-sequence (tokenize "func main int { return 0; }"))) (make-env))))))

(fiveam:test test-return-stmt
  (fiveam:is (null (assign-type (stmt-rule (make-token-sequence (tokenize "return 0;"))) (make-env :return-type (make-integer-type :size :int32)))))
  (fiveam:signals error (assign-type (stmt-rule (make-token-sequence (tokenize "return 0;"))) (make-env))))

(fiveam:test test-expr
  (fiveam:is (eq :generic (integer-type-size (assign-type (expr-bp (make-token-sequence (tokenize "0")) 0) (make-env)))))
  (fiveam:is (eq :int32 (integer-type-size (assign-type (expr-bp (make-token-sequence (tokenize "(int) 0")) 0) (make-env))))))