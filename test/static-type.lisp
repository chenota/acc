(in-package :acc/test)

(fiveam:def-suite static-type)
(fiveam:in-suite static-type)

(fiveam:test test-program
  (fiveam:is (acc::set-program-types (acc::parse-program (acc::make-token-sequence (acc::tokenize "fun main int { return 0; }"))))))

(fiveam:test test-function
  (fiveam:is (null (acc::function-type-parameters (acc::ast-node-type-info (acc::assign-type (acc::function-rule (acc::make-token-sequence (acc::tokenize "fun main int { return 0; }"))) (acc::make-env))))))
  (fiveam:is (acc::integer-type-p (acc::function-type-return-type (acc::ast-node-type-info (acc::assign-type (acc::function-rule (acc::make-token-sequence (acc::tokenize "fun main int { return 0; }"))) (acc::make-env)))))))

(fiveam:test test-return-stmt
  (fiveam:is (eq :int32 (acc::integer-type-size (acc::ast-node-type-info (acc::return-statement-node-expression (acc::assign-type (acc::stmt-rule (acc::make-token-sequence (acc::tokenize "return 0;"))) (acc::make-env :return-type (acc::make-integer-type :size :int32))))))))
  (fiveam:signals error (acc::assign-type (acc::stmt-rule (acc::make-token-sequence (acc::tokenize "return 0;"))) (acc::make-env))))

(fiveam:test test-expr
  (fiveam:is (eq :generic (acc::integer-type-size (acc::ast-node-type-info (acc::assign-type (acc::expr-bp (acc::make-token-sequence (acc::tokenize "0")) 0) (acc::make-env))))))
  (fiveam:is (acc::int-node-p (acc::assign-type (acc::expr-bp (acc::make-token-sequence (acc::tokenize "(int) 0")) 0) (acc::make-env))))
  (fiveam:is (eq :int8 (acc::integer-type-size (acc::ast-node-type-info (acc::assign-type (acc::expr-bp (acc::make-token-sequence (acc::tokenize "(int8) 0")) 0) (acc::make-env)))))))

(fiveam:test cast-overflow-err
  (fiveam:signals error (acc::assign-type (acc::expr-bp (acc::make-token-sequence (acc::tokenize "(int8) 256")) 0) (acc::make-env)))
  (fiveam:signals error (acc::assign-type (acc::expr-bp (acc::make-token-sequence (acc::tokenize (format nil "(int16) ~A" (ash 1 16)))) 0) (acc::make-env)))
  (fiveam:signals error (acc::assign-type (acc::expr-bp (acc::make-token-sequence (acc::tokenize (format nil "(int32) ~A" (ash 1 32)))) 0) (acc::make-env)))
  (fiveam:signals error (acc::assign-type (acc::expr-bp (acc::make-token-sequence (acc::tokenize (format nil "(int64) ~A" (ash 1 64)))) 0) (acc::make-env))))