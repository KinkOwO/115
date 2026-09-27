// ui_1441ECBC0

__int64 __fastcall sub_1441ECBC0(__int64 a1)
{
  __int64 v2; // rcx
  __int64 v3; // rax
  void (__fastcall ***v4)(_QWORD); // rcx
  __int64 result; // rax
  __int64 v6; // rdx
  __int64 v7; // rdx
  __int64 v8; // rdx
  __int64 v9; // rcx
  __int64 v10; // r8
  __int64 v11; // rax
  __int64 v12; // rcx
  _QWORD *v13; // rax
  unsigned __int8 v14; // al
  __int64 v15; // rax
  unsigned __int8 v16; // bl
  int v17; // eax
  __int64 v18; // rcx
  _DWORD *v19; // rsi
  __int64 v20; // rbx
  __int64 *v21; // rax
  unsigned __int8 v22; // al
  __int64 v23; // [rsp+50h] [rbp+8h] BYREF

  v2 = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    v3 = sub_146E8BA20(1472);
    v23 = v3;
    if ( v3 )
      v4 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v3);
    else
      v4 = 0;
    qword_14E638F28 = (__int64)v4;
    (**v4)(v4);
    v2 = qword_14E638F28;
  }
  result = (__int64)sub_1444EBD90(v2, 9, *(_DWORD *)(a1 + 40));
  if ( result )
  {
    if ( sub_1444EC2C0(result) )
    {
      (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 304) + 16LL))(*(_QWORD *)(a1 + 304), 0);
      (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 192) + 16LL))(*(_QWORD *)(a1 + 192), 0);
      (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 208) + 16LL))(*(_QWORD *)(a1 + 208), 0);
      (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 224) + 16LL))(*(_QWORD *)(a1 + 224), 0);
      (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 240) + 16LL))(*(_QWORD *)(a1 + 240), 0);
    }
    else
    {
      LOBYTE(v6) = 1;
      (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 224) + 16LL))(*(_QWORD *)(a1 + 224), v6);
      LOBYTE(v7) = 1;
      (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 304) + 16LL))(*(_QWORD *)(a1 + 304), v7);
      v11 = sub_1401DCCB0(v9, v8, v10);
      if ( (unsigned __int16)sub_1403F5F60(v11, 169) )
      {
        v13 = (_QWORD *)sub_140764510(v12);
        v14 = sub_1444EC370(v13, *(_DWORD *)(a1 + 40));
      }
      else
      {
        v15 = sub_140764510(v12);
        v14 = sub_1444EC410(v15, *(_DWORD *)(a1 + 40));
      }
      v16 = v14;
      (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 240) + 16LL))(*(_QWORD *)(a1 + 240), v14);
      (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 192) + 16LL))(*(_QWORD *)(a1 + 192), v16 ^ 1u);
      (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 208) + 16LL))(*(_QWORD *)(a1 + 208), v16);
      sub_146F01920(*(_QWORD *)(a1 + 304), v16);
    }
    v17 = sub_14667BB90(qword_14E683C78, 730, 0);
    result = sub_148AA307C(v17, 0, (unsigned int)&off_14DCB4760, (unsigned int)&off_14DD262B0, 0);
    v19 = (_DWORD *)result;
    if ( result )
    {
      v20 = sub_140764510(v18);
      v21 = (__int64 *)sub_1441C3DB0(v19, (__int64)&v23);
      v22 = sub_1444EC320(v20, *(_DWORD *)(a1 + 40), *v21);
      return (*(__int64 (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 24) + 16LL))(*(_QWORD *)(a1 + 24), v22);
    }
  }
  return result;
}

