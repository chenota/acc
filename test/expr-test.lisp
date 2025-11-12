(in-package :acc)

(fiveam:def-suite expr)

(test test-parse-int
  (is (eq :int (car (expr-bp (make-token-sequence (tokenize "100")) 0))))
  (is (= 100 (cdr (expr-bp (make-token-sequence (tokenize "100")) 0)))))

(test test-parse-err
  (signals error (expr-bp (make-token-sequence (tokenize "return")))))