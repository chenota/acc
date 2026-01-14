(in-package :acc)

(fiveam:def-suite expr)

(fiveam:test test-parse-int
  (fiveam:is (eq :int (car (expr-bp (make-token-sequence (tokenize "100")) 0))))
  (fiveam:is (= 100 (cadr (expr-bp (make-token-sequence (tokenize "100")) 0)))))

(fiveam:test test-parse-err
  (fiveam:signals error (expr-bp (make-token-sequence (tokenize "return")) 0)))