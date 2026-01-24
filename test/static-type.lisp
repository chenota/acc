(in-package :acc)

(fiveam:def-suite static-type)

(fiveam:test test-program
  (fiveam:is (let ((ast (parse-program (make-token-sequence (tokenize "func main int { return 0; }")))))
               (assign-types ast nil)
               (function-type-p (ast-node-type-info (first (program-node-functions ast)))))))

(fiveam:test test-function
  (fiveam:is (let ((ast (function-rule (make-token-sequence (tokenize "func main int { return 0; }")))))
               (assign-types ast nil)
               (null (function-type-parameters (ast-node-type-info ast)))))
  (fiveam:is (let ((ast (function-rule (make-token-sequence (tokenize "func main int { return 0; }")))))
               (assign-types ast nil)
               (primitive-type-p (function-type-return-type (ast-node-type-info ast))))))

(fiveam:test test-return-stmt
  (fiveam:is (let ((ast (stmt-rule (make-token-sequence (tokenize "return 0;")))))
               (assign-types ast :int)
               (primitive-type-p (ast-node-type-info (return-statement-node-expression ast)))))
  (fiveam:signals error (assign-types (stmt-rule (make-token-sequence (tokenize "return 0;"))) nil)))

(fiveam:test test-expr
  (fiveam:is (let ((ast (expr-bp (make-token-sequence (tokenize "0")) 0)))
               (assign-types ast nil)
               (eq :untyped-int (primitive-type-kind (ast-node-type-info ast)))))
  (fiveam:is (let ((ast (expr-bp (make-token-sequence (tokenize "(int) 0")) 0)))
               (assign-types ast nil)
               (eq :int32 (primitive-type-kind (ast-node-type-info ast))))))