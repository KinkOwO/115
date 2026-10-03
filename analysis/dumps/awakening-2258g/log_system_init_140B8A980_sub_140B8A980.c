// sub_140B8A980  va=0x140B8A980  size=414

__int64 __fastcall sub_140B8A980(__int64 a1)
{
  void (__fastcall ***v2)(_QWORD); // rsi
  __int64 v3; // rbp
  __int64 v4; // rax
  void (__fastcall ***v5)(_QWORD); // rcx
  __int64 v6; // rbx
  __int64 v7; // rdi
  __int64 v8; // rax
  __int64 v9; // rax
  __int64 result; // rax
  __int64 v11; // rbp
  __int64 v12; // rax
  __int64 v13; // rbx
  __int64 v14; // rdi
  __int64 v15; // rax
  __int64 v16; // [rsp+70h] [rbp+8h]
  __int64 v17; // [rsp+70h] [rbp+8h]

  sub_140B89AB0();
  v2 = nullptr;
  if ( (unsigned __int8)sub_1477E07E0(a1 + 8) == 0 )
  {
    v3 = *(_QWORD *)&qword_14E6343D0;
    if ( *(_QWORD *)&qword_14E6343D0 == 0 )
    {
      v4 = sub_146E8BA20(72);
      v16 = v4;
      __wind
      {
        if ( v4 != 0 )
          v5 = (void (__fastcall ***)(_QWORD))sub_146E93360(v4);
        else
          v5 = nullptr;
      }
      __unwind
      {
        j_j_scalable_free(v16, 72);
      }
      *(_QWORD *)&qword_14E6343D0 = v5;
      (**v5)(v5);
      v3 = *(_QWORD *)&qword_14E6343D0;
    }
    v6 = sub_146E8C7D0(&unk_149483BD0);
    v7 = sub_146E8C7D0(&unk_149483C30);
    v8 = sub_146E8C7D0(&unk_149483C90);
    sub_146E939E0(v3, 0, v8, v7, 118, &qword_14E639988, v6);
  }
  v9 = sub_146E8C7D0(&unk_149483D30);
  result = sub_1470CD480(a1 + 64, &Source, v9, 0);
  if ( (_BYTE)result == 0 )
  {
    v11 = *(_QWORD *)&qword_14E6343D0;
    if ( *(_QWORD *)&qword_14E6343D0 == 0 )
    {
      v12 = sub_146E8BA20(72);
      v17 = v12;
      __wind
      {
        if ( v12 != 0 )
          v2 = (void (__fastcall ***)(_QWORD))sub_146E93360(v12);
      }
      __unwind
      {
        j_j_scalable_free(v17, 72);
      }
      *(_QWORD *)&qword_14E6343D0 = v2;
      (**v2)(v2);
      v11 = *(_QWORD *)&qword_14E6343D0;
    }
    v13 = sub_146E8C7D0(&unk_149483DA0);
    v14 = sub_146E8C7D0(&unk_149483C30);
    v15 = sub_146E8C7D0(&unk_149483C90);
    return sub_146E939E0(v11, 0, v15, v14, 122, &qword_14E639988, v13);
  }
  return result;
}
