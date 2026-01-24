(in-package :acc)

(fiveam:def-suite expr)

(fiveam:test test-parse-int
  (fiveam:is (int-node-p (expr-bp (make-token-sequence (tokenize "100")) 0)))
  (fiveam:is (= 100 (int-node-value (expr-bp (make-token-sequence (tokenize "100")) 0)))))

(fiveam:test test-parse-err
  (fiveam:signals error (expr-bp (make-token-sequence (tokenize "return")) 0)))

(fiveam:test test-parse-cast
  (fiveam:is (cast-node-p (expr-bp (make-token-sequence (tokenize "(int) 100")) 0)))
  (fiveam:is (eq :char (integer-type-size (cast-node-cast-type (expr-bp (make-token-sequence (tokenize "(char) (int) 100")) 0)))))
  (fiveam:is (eq :int32 (integer-type-size (cast-node-cast-type (cast-node-expression (expr-bp (make-token-sequence (tokenize "(char) (int) 100")) 0))))))
  (fiveam:signals error (expr-bp (make-token-sequence (tokenize "(int 100")) 0))
  (fiveam:signals error (expr-bp (make-token-sequence (tokenize "int) 100")) 0))
  (fiveam:signals error (expr-bp (make-token-sequence (tokenize "((int)) 100")) 0)))

(fiveam:test test-grouped-expr
  (fiveam:is (int-node-p (expr-bp (make-token-sequence (tokenize "(100)")) 0)))
  (fiveam:is (int-node-p (expr-bp (make-token-sequence (tokenize "(((100)))")) 0)))
  (fiveam:signals error (expr-bp (make-token-sequence (tokenize "(100")) 0))
  (fiveam:signals error (expr-bp (make-token-sequence (tokenize "((100)")) 0)))