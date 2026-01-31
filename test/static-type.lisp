(in-package :acc)

(fiveam:def-suite static-type)

(fiveam:test test-program
  (fiveam:is (set-program-types (parse-program (make-token-sequence (tokenize "func main int { return 0; }"))))))

(fiveam:test test-function
  (fiveam:is (null (function-type-parameters (ast-node-type-info (assign-type (function-rule (make-token-sequence (tokenize "func main int { return 0; }"))) (make-env))))))
  (fiveam:is (integer-type-p (function-type-return-type (ast-node-type-info (assign-type (function-rule (make-token-sequence (tokenize "func main int { return 0; }"))) (make-env)))))))

(fiveam:test test-return-stmt
  (fiveam:is (eq :int32 (integer-type-size (ast-node-type-info (return-statement-node-expression (assign-type (stmt-rule (make-token-sequence (tokenize "return 0;"))) (make-env :return-type (make-integer-type :size :int32))))))))
  (fiveam:signals error (assign-type (stmt-rule (make-token-sequence (tokenize "return 0;"))) (make-env))))

(fiveam:test test-expr
  (fiveam:is (eq :generic (integer-type-size (ast-node-type-info (assign-type (expr-bp (make-token-sequence (tokenize "0")) 0) (make-env))))))
  (fiveam:is (int-node-p (assign-type (expr-bp (make-token-sequence (tokenize "(int) 0")) 0) (make-env))))
  (fiveam:is (eq :int8 (integer-type-size (ast-node-type-info (assign-type (expr-bp (make-token-sequence (tokenize "(int8) 0")) 0) (make-env)))))))

(fiveam:test cast-overflow-err
  (fiveam:signals error (assign-type (expr-bp (make-token-sequence (tokenize "(int8) 256")) 0) (make-env)))
  (fiveam:signals error (assign-type (expr-bp (make-token-sequence (tokenize (format nil "(int16) ~A" (ash 1 16)))) 0) (make-env)))
  (fiveam:signals error (assign-type (expr-bp (make-token-sequence (tokenize (format nil "(int32) ~A" (ash 1 32)))) 0) (make-env)))
  (fiveam:signals error (assign-type (expr-bp (make-token-sequence (tokenize (format nil "(int64) ~A" (ash 1 64)))) 0) (make-env))))