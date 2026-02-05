(in-package :acc/test)

(fiveam:def-suite expr)
(fiveam:in-suite expr)

(fiveam:test test-parse-atom
  (fiveam:is (acc::int-node-p (acc::expr-bp (acc::make-token-sequence (acc::tokenize "100")) 0)))
  (fiveam:is (= 100 (acc::int-node-value (acc::expr-bp (acc::make-token-sequence (acc::tokenize "100")) 0))))
  (fiveam:is (acc::ident-node-p (acc::expr-bp (acc::make-token-sequence (acc::tokenize "x")) 0)))
  (fiveam:is (string= "x" (acc::ident-node-name (acc::expr-bp (acc::make-token-sequence (acc::tokenize "x")) 0)))))

(fiveam:test test-parse-err
  (fiveam:signals error (acc::expr-bp (acc::make-token-sequence (acc::tokenize "return")) 0)))

(fiveam:test test-parse-cast
  (fiveam:is (acc::cast-node-p (acc::expr-bp (acc::make-token-sequence (acc::tokenize "(int) 100")) 0)))
  (fiveam:is (eq :int8 (acc::integer-type-size (acc::cast-node-cast-type (acc::expr-bp (acc::make-token-sequence (acc::tokenize "(int8) (int) 100")) 0)))))
  (fiveam:is (eq :int32 (acc::integer-type-size (acc::cast-node-cast-type (acc::cast-node-expression (acc::expr-bp (acc::make-token-sequence (acc::tokenize "(int8) (int) 100")) 0)))))))

(fiveam:test test-cast-missing-closing-paren
  (fiveam:signals error (acc::expr-bp (acc::make-token-sequence (acc::tokenize "(int 100")) 0)))

(fiveam:test test-cast-too-many-parens
  (fiveam:signals error (acc::expr-bp (acc::make-token-sequence (acc::tokenize "((int)) 100")) 0)))

(fiveam:test test-grouped-expr
  (fiveam:is (acc::int-node-p (acc::expr-bp (acc::make-token-sequence (acc::tokenize "((100))")) 0))))

(fiveam:test test-group-missing-paren
  (fiveam:signals error (acc::expr-bp (acc::make-token-sequence (acc::tokenize "((100)")) 0)))