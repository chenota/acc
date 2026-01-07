(in-package :acc)

(fiveam:def-suite lexer)

(fiveam:test kw-test-single
  (fiveam:is (eq :func (token-kind (first (tokenize "func")))))
  (fiveam:is (eq :semi (token-kind (first (tokenize ";")))))
  (fiveam:is (eq :return (token-kind (first (tokenize "return"))))))

(fiveam:test kw-test-multi
  (fiveam:is (equal '(:func :lbrace :return :rbrace) (mapcar #'token-kind (tokenize "func{return}"))))
  (fiveam:is (equal '(:lbrace :rbrace :lbrace :rbrace) (mapcar #'token-kind (tokenize "{}{}"))))
  (fiveam:is (equal '(:return :lbrace :func :rbrace) (mapcar #'token-kind (tokenize "return{func}")))))

(fiveam:test ignore-whitespace
  (fiveam:is (equal '(:return :func :return :func) (mapcar #'token-kind (tokenize "return func return func")))))

(fiveam:test int
  (fiveam:is (equal '(100 10 45 3) (mapcar #'token-value (tokenize "100 10 45 3"))))
  (fiveam:is (equal '(:int :int :int :int) (mapcar #'token-kind (tokenize "100 10 45 3"))))
  (fiveam:is (equal '(3 2 2 1) (mapcar #'token-len (tokenize "100 10 45 3")))))

(fiveam:test ident
  (fiveam:is (equal '("abc" "helloworld" "x") (mapcar #'token-value (tokenize "abc helloworld x"))))
  (fiveam:is (equal '(:ident :ident :ident) (mapcar #'token-kind (tokenize "abc helloworld x"))))
  (fiveam:is (equal '(3 10 1) (mapcar #'token-len (tokenize "abc helloworld x"))))
  (fiveam:is (equal '("returnreturn" "funcreturn") (mapcar #'token-value (tokenize "returnreturn funcreturn")))))

(fiveam:test location
  (fiveam:is (equal '(0 2 6) (mapcar #'token-row (tokenize "a bcd efgh"))))
  (fiveam:is (equal '(0 0 0) (mapcar #'token-col (tokenize "a bcd efgh"))))
  (fiveam:is (equal '(0 0 4 0) (mapcar #'token-row (tokenize (format nil "a~%bcd efg~%hij")))))
  (fiveam:is (equal '(0 1 1 2) (mapcar #'token-col (tokenize (format nil "a~%bcd efg~%hij"))))))