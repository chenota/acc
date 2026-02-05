(in-package :acc)

(fiveam:def-suite expr)
(fiveam:in-suite expr)

(fiveam:test test-parse-atom
  (fiveam:is (int-node-p (expr-bp (make-token-sequence (tokenize "100")) 0)))
  (fiveam:is (= 100 (int-node-value (expr-bp (make-token-sequence (tokenize "100")) 0))))
  (fiveam:is (ident-node-p (expr-bp (make-token-sequence (tokenize "x")) 0)))
  (fiveam:is (string= "x" (ident-node-name (expr-bp (make-token-sequence (tokenize "x")) 0)))))

(fiveam:test test-parse-err
  (fiveam:signals error (expr-bp (make-token-sequence (tokenize "return")) 0)))

(fiveam:test test-parse-cast
  (fiveam:is (cast-node-p (expr-bp (make-token-sequence (tokenize "(int) 100")) 0)))
  (fiveam:is (eq :int8 (integer-type-size (cast-node-cast-type (expr-bp (make-token-sequence (tokenize "(int8) (int) 100")) 0)))))
  (fiveam:is (eq :int32 (integer-type-size (cast-node-cast-type (cast-node-expression (expr-bp (make-token-sequence (tokenize "(int8) (int) 100")) 0)))))))

(fiveam:test test-cast-missing-closing-paren
  (fiveam:signals error (expr-bp (make-token-sequence (tokenize "(int 100")) 0)))

(fiveam:test test-cast-too-many-parens
  (fiveam:signals error (expr-bp (make-token-sequence (tokenize "((int)) 100")) 0)))

(fiveam:test test-grouped-expr
  (fiveam:is (int-node-p (expr-bp (make-token-sequence (tokenize "((100))")) 0))))

(fiveam:test test-group-missing-paren
  (fiveam:signals error (expr-bp (make-token-sequence (tokenize "((100)")) 0)))