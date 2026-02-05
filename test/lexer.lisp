(in-package :acc/test)

(fiveam:def-suite lexer)
(fiveam:in-suite lexer)

(fiveam:test kw-test-single
  (fiveam:is (eq :fun (acc::token-kind (first (acc::tokenize "fun")))))
  (fiveam:is (eq :semi (acc::token-kind (first (acc::tokenize ";")))))
  (fiveam:is (eq :return (acc::token-kind (first (acc::tokenize "return"))))))

(fiveam:test kw-test-multi
  (fiveam:is (equal '(:fun :lbrace :return :rbrace) (mapcar #'acc::token-kind (acc::tokenize "fun{return}"))))
  (fiveam:is (equal '(:lbrace :rbrace :lbrace :rbrace) (mapcar #'acc::token-kind (acc::tokenize "{}{}"))))
  (fiveam:is (equal '(:return :lbrace :fun :rbrace) (mapcar #'acc::token-kind (acc::tokenize "return{fun}")))))

(fiveam:test ignore-whitespace
  (fiveam:is (equal '(:return :fun :return :fun) (mapcar #'acc::token-kind (acc::tokenize "return fun return fun")))))

(fiveam:test int
  (fiveam:is (equal '(100 10 45 3) (mapcar #'acc::token-value (acc::tokenize "100 10 45 3"))))
  (fiveam:is (equal '(:int :int :int :int) (mapcar #'acc::token-kind (acc::tokenize "100 10 45 3"))))
  (fiveam:is (equal '(3 2 2 1) (mapcar #'acc::token-len (acc::tokenize "100 10 45 3")))))

(fiveam:test ident
  (fiveam:is (equal '("abc" "helloworld" "x") (mapcar #'acc::token-value (acc::tokenize "abc helloworld x"))))
  (fiveam:is (every (lambda (x) (eq (acc::token-kind x) :ident)) (acc::tokenize "abc helloworld x int64")))
  (fiveam:is (equal '(3 10 1) (mapcar #'acc::token-len (acc::tokenize "abc helloworld x"))))
  (fiveam:is (equal '("returnreturn" "funcreturn") (mapcar #'acc::token-value (acc::tokenize "returnreturn funcreturn")))))

(fiveam:test location
  (fiveam:is (equal '(0 2 6) (mapcar (lambda (x) (first (acc::token-loc x))) (acc::tokenize "a bcd efgh"))))
  (fiveam:is (equal '(0 0 0) (mapcar (lambda (x) (second (acc::token-loc x))) (acc::tokenize "a bcd efgh"))))
  (fiveam:is (equal '(0 0 4 0) (mapcar (lambda (x) (first (acc::token-loc x))) (acc::tokenize (format nil "a~%bcd efg~%hij")))))
  (fiveam:is (equal '(0 1 1 2) (mapcar (lambda (x) (second (acc::token-loc x))) (acc::tokenize (format nil "a~%bcd efg~%hij"))))))