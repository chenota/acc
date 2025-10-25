(in-package :acc)

(fiveam:def-suite lexer)

(test kw-test-single
  (is (eq :func (token-kind (first (tokenize "func")))))
  (is (eq :semi (token-kind (first (tokenize ";")))))
  (is (eq :return (token-kind (first (tokenize "return"))))))

(test kw-test-multi
  (is (equal '(:func :lbrace :return :rbrace) (mapcar #'token-kind (tokenize "func{return}"))))
  (is (equal '(:lbrace :rbrace :lbrace :rbrace) (mapcar #'token-kind (tokenize "{}{}"))))
  (is (equal '(:return :lbrace :func :rbrace) (mapcar #'token-kind (tokenize "return{func}")))))

(test ignore-whitespace
  (is (equal '(:return :func :return :func) (mapcar #'token-kind (tokenize "return func return func")))))

(test int
  (is (equal '(100 10 45 3) (mapcar #'token-value (tokenize "100 10 45 3"))))
  (is (equal '(:int :int :int :int) (mapcar #'token-kind (tokenize "100 10 45 3"))))
  (is (equal '(3 2 2 1) (mapcar #'token-len (tokenize "100 10 45 3")))))

(test ident
  (is (equal '("abc" "helloworld" "x") (mapcar #'token-value (tokenize "abc helloworld x"))))
  (is (equal '(:ident :ident :ident) (mapcar #'token-kind (tokenize "abc helloworld x"))))
  (is (equal '(3 10 1) (mapcar #'token-len (tokenize "abc helloworld x"))))
  (is (equal '("returnreturn" "funcreturn") (mapcar #'token-value (tokenize "returnreturn funcreturn")))))

(test location
  (is (equal '(0 2 6) (mapcar #'token-row (tokenize "a bcd efgh"))))
  (is (equal '(0 0 0) (mapcar #'token-col (tokenize "a bcd efgh"))))
  (is (equal '(0 0 4 0) (mapcar #'token-row (tokenize (format nil "a~%bcd efg~%hij")))))
  (is (equal '(0 1 1 2) (mapcar #'token-col (tokenize (format nil "a~%bcd efg~%hij"))))))