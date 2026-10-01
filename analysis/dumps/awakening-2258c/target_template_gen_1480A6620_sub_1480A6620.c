// sub_1480A6620  va=0x1480A6620  size=392

_DWORD *__fastcall sub_1480A6620(__int64 a1, __int64 a2, char a3, char a4, _DWORD *a5)
{
  unsigned __int64 v5; // rbx
  int v7; // ebp
  _DWORD *result; // rax
  _QWORD *v10; // rcx
  _DWORD *v11; // rcx
  __int64 v12; // [rsp+20h] [rbp-38h] BYREF
  _BYTE v13[48]; // [rsp+28h] [rbp-30h] BYREF
  int v14; // [rsp+60h] [rbp+8h] BYREF

  v5 = a1 ^ 0x7F89372C7F89372CLL;
  v7 = 0;
  v12 = a1 ^ 0x7F89372C7F89372CLL;
  v14 = 0;
  if ( (unsigned __int8)sub_1401BDD40(a1, a2, &v14) != 0 )
    v7 = v14;
  result = a5;
  if ( *a5 != v7 )
  {
    v10 = (_QWORD *)(qword_14F3B8E48
                   + 16
                   * ((0x100000001B3LL
                     * (HIBYTE(v5)
                      ^ (0x100000001B3LL
                       * (BYTE6(v5)
                        ^ (0x100000001B3LL
                         * (BYTE5(v5)
                          ^ (0x100000001B3LL
                           * (BYTE4(v5)
                            ^ (0x100000001B3LL
                             * (BYTE3(v5)
                              ^ (0x100000001B3LL
                               * (BYTE2(v5)
                                ^ (0x100000001B3LL
                                 * (BYTE1(v5) ^ (0x100000001B3LL * ((unsigned __int8)v5 ^ 0xCBF29CE484222325uLL))))))))))))))))
                    & qword_14F3B8E60));
    result = (_DWORD *)v10[1];
    if ( result == (_DWORD *)qword_14F3B8E38 )
      goto LABEL_10;
    v11 = (_DWORD *)*v10;
    if ( v5 != *((_QWORD *)result + 2) )
    {
      while ( result != v11 )
      {
        result = *((_DWORD **)result + 1);
        if ( v5 == *((_QWORD *)result + 2) )
          goto LABEL_8;
      }
      goto LABEL_10;
    }
LABEL_8:
    if ( result == nullptr || result == (_DWORD *)qword_14F3B8E38 )
    {
LABEL_10:
      LOBYTE(a5) = a3;
      BYTE1(a5) = a4;
      qword_14F3B8E20(&a5);
      return (_DWORD *)sub_1421CFB50(&dword_14F3B8E30, v13, &v12);
    }
  }
  return result;
}
