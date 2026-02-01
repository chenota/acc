(in-package :acc)

;; HELPER MACROS

(with-ignore-coverage
  (defmacro def-atomic-operand (name control-string value-type)
    "Create a new atomic operand."
    (let* ((pkg (symbol-package name))
           (make-fun (intern (format nil "MAKE-~A" (symbol-name name)) pkg)))
      `(progn
        (defclass ,name (atomic-operand) ())
        (defmethod control-string ((_ ,name)) (declare (ignore _)) ,control-string)
        (defmethod value-type ((_ ,name)) (declare (ignore _)) ',value-type)
        (defun ,make-fun (value) (make-instance ',name :value value))))))

(with-ignore-coverage
  (defmacro def-register-operand (name register-list)
    "Create a new register operand."
    (let* ((pkg (symbol-package name))
           (make-fun (intern (format nil "MAKE-~A" (symbol-name name)) pkg)))
      `(progn
        (defclass ,name (register-operand) ())
        (defmethod register-list ((_ ,name)) (declare (ignore _)) ,register-list)
        (defun ,make-fun (i) (make-instance ',name :i i))))))

;; REGISTER LISTS

(with-ignore-coverage
  (defparameter
    +gpreg8-list+
    #("al"
      "bl"
      "cl"
      "dl"
      "sil"
      "dil"
      "bpl"
      "spl"
      "r8b"
      "r9b"
      "r10b"
      "r11b"
      "r12b"
      "r13b"
      "r14b"
      "r15b"))

  (defparameter
    +gpreg16-list+
    #("ax"
      "bx"
      "cx"
      "dx"
      "si"
      "di"
      "bp"
      "sp"
      "r8w"
      "r9w"
      "r10w"
      "r11w"
      "r12w"
      "r13w"
      "r14w"
      "r15w"))

  (defparameter
    +gpreg32-list+
    #("eax"
      "ebx"
      "ecx"
      "edx"
      "esi"
      "edi"
      "esp"
      "ebp"
      "r8d"
      "r9d"
      "r10d"
      "r11d"
      "r12d"
      "r13d"
      "r14d"
      "r15d"))

  (defparameter
    +gpreg64-list+
    #("rax"
      "rbx"
      "rcx"
      "rdx"
      "rsi"
      "rdi"
      "rsp"
      "rbp"
      "r8"
      "r9"
      "r10"
      "r11"
      "r12"
      "r13"
      "r14"
      "r15")))

;; OPERANDS

(with-ignore-coverage
  (defclass operand () ()
    (:documentation "Operand base class.")))

(defmethod print-operand :before ((o operand) s)
  "Assert the printer function writes to a stream."
  (assert (typep s 'stream)))

(defmethod to-string ((o operand))
  "Convert operand to a string."
  (with-output-to-string (s)
    (print-operand o s)))

(with-ignore-coverage
  (defclass atomic-operand (operand)
      ((value :initarg :value :accessor operand-value))
    (:documentation "Operand with a single item of structured data.")))

(defmethod initialize-instance :before ((a atomic-operand) &key value &allow-other-keys)
  "Assert the type of the atomic operands value."
  (assert (typep value (value-type a))))

(defmethod print-operand ((a atomic-operand) s)
  "Print the atomic operand."
  (format s (control-string a) (operand-value a)))

(def-atomic-operand string-operand "~s" string)
(def-atomic-operand ident-operand "~a" string)
(def-atomic-operand type-operand "@~a" string)
(def-atomic-operand immediate-operand "$~D" integer)
(def-atomic-operand number-operand "~D" integer)

(with-ignore-coverage
  (defclass register-operand (operand)
      ((i :initarg :i :accessor register-operand-i :type (integer 0 *)))
    (:documentation "Register operand.")))

(defmethod initialize-instance :before ((r register-operand) &key i &allow-other-keys)
  "Assert the bounds of the register index."
  (assert (>= i 0))
  (assert (< i (length (register-list r)))))

(defmethod print-operand ((r register-operand) s)
  "Print the register operand."
  (format s "%~a" (aref (register-list r) (register-operand-i r))))

(with-ignore-coverage
  (def-register-operand gpreg8-operand +gpreg8-list+)
  (def-register-operand gpreg16-operand +gpreg16-list+)
  (def-register-operand gpreg32-operand +gpreg32-list+)
  (def-register-operand gpreg64-operand +gpreg64-list+))

;; INSTRUCTIONS

(with-ignore-coverage
  (defclass line-item () ()))

(defmethod to-string ((l line-item))
  "Convert line item to a string."
  (with-output-to-string (s)
    (print-line-item l s)))

(defmethod print-line-item :before ((l line-item) s)
  "Assert the line item writes to a stream."
  (assert (typep s 'stream)))

(with-ignore-coverage
  (defclass instruction (line-item)
      ((operation :initarg :op :type string :accessor instr-op)
       (operands :initarg :oprs :type sequence :accessor instr-oprs :initform nil))
    (:documentation "Basic x86 instruction with a type and operands.")))

(defun make-instruction (operator &rest operands)
  (make-instance 'instruction :op operator :oprs operands))

(defmethod print-line-item ((i instruction) s)
  (format s "~c~a" #\tab (instr-op i))
  (let ((opr-count (length (instr-oprs i))))
    (when (> opr-count 0) (write-string " " s))
    (loop
   for operand in (instr-oprs i)
   for j = 0 then (1+ j)
   do (print-operand operand s)
     when (< j (1- opr-count)) do (write-string ", " s))))

(defmethod initialize-instance :before ((i instruction) &key oprs &allow-other-keys)
  "Assert that the instruction operands are only of the operand type."
  (assert (every (lambda (x) (typep x 'operand)) oprs)))

(with-ignore-coverage
  (defclass label (line-item)
      ((value :initarg :value :type string :accessor label-value))
    (:documentation "x86 label.")))

(defun make-label (value)
  (make-instance 'label :value value))

(defmethod print-line-item ((l label) s)
  (format s "~a:" (label-value l)))