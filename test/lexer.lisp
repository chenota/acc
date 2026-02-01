(in-package :acc)

(fiveam:def-suite lexer)

(fiveam:test kw-test-single
  (fiveam:is (eq :fun (token-kind (first (tokenize "fun")))))
  (fiveam:is (eq :semi (token-kind (first (tokenize ";")))))
  (fiveam:is (eq :return (token-kind (first (tokenize "return"))))))

(fiveam:test kw-test-multi
  (fiveam:is (equal '(:fun :lbrace :return :rbrace) (mapcar #'token-kind (tokenize "fun{return}"))))
  (fiveam:is (equal '(:lbrace :rbrace :lbrace :rbrace) (mapcar #'token-kind (tokenize "{}{}"))))
  (fiveam:is (equal '(:return :lbrace :fun :rbrace) (mapcar #'token-kind (tokenize "return{fun}")))))

(fiveam:test ignore-whitespace
  (fiveam:is (equal '(:return :fun :return :fun) (mapcar #'token-kind (tokenize "return fun return fun")))))

(fiveam:test int
  (fiveam:is (equal '(100 10 45 3) (mapcar #'token-value (tokenize "100 10 45 3"))))
  (fiveam:is (equal '(:int :int :int :int) (mapcar #'token-kind (tokenize "100 10 45 3"))))
  (fiveam:is (equal '(3 2 2 1) (mapcar #'token-len (tokenize "100 10 45 3")))))

(fiveam:test ident
  (fiveam:is (equal '("abc" "helloworld" "x") (mapcar #'token-value (tokenize "abc helloworld x"))))
  (fiveam:is (every (lambda (x) (eq (token-kind x) :ident)) (tokenize "abc helloworld x int64")))
  (fiveam:is (equal '(3 10 1) (mapcar #'token-len (tokenize "abc helloworld x"))))
  (fiveam:is (equal '("returnreturn" "funcreturn") (mapcar #'token-value (tokenize "returnreturn funcreturn")))))

(fiveam:test location
  (fiveam:is (equal '(0 2 6) (mapcar (lambda (x) (first (token-loc x))) (tokenize "a bcd efgh"))))
  (fiveam:is (equal '(0 0 0) (mapcar (lambda (x) (second (token-loc x))) (tokenize "a bcd efgh"))))
  (fiveam:is (equal '(0 0 4 0) (mapcar (lambda (x) (first (token-loc x))) (tokenize (format nil "a~%bcd efg~%hij")))))
  (fiveam:is (equal '(0 1 1 2) (mapcar (lambda (x) (second (token-loc x))) (tokenize (format nil "a~%bcd efg~%hij"))))))