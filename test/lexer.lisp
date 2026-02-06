(in-package :acc/test)

(fiveam:def-suite lexer)
(fiveam:in-suite lexer)

(fiveam:test kw-single (fiveam:is (eq :fun (acc::token-kind (first (acc::tokenize "fun"))))))

(fiveam:test mixed-multi (fiveam:is (equal (list :ident :int :fun) (mapcar #'acc::token-kind (acc::tokenize "a 10 fun")))))

(fiveam:test location-row (fiveam:is (equal '(0 1 1) (mapcar (lambda (x) (first (acc::token-loc x))) (acc::tokenize (format nil "a~%b c"))))))

(fiveam:test location-col (fiveam:is (equal '(0 0 2) (mapcar (lambda (x) (second (acc::token-loc x))) (acc::tokenize (format nil "a~%b c"))))))